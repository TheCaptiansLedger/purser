// End-to-end music import flow — two canonical import paths:
//
//  1. REO Speedwagon — Hi Infidelity (1981)
//     Barcode hit + ISRCs present → high confidence → auto-import path.
//     Signals: barcode=1.0, isrc=0.95, rgNameFuzzy=0.62, duration=0.88
//     Queue entry is dismissed after import completes.
//
//  2. Stevie Nicks — The Enchanted Works of Stevie Nicks (1998, 3-disc)
//     No barcode, no ISRCs, no MBZ IDs in tags → fuzzy name + track count + duration only.
//     Signals: barcode=0, isrc=0, rgNameFuzzy=0.62, trackCount=0.20, trackTitleSet=0.45, duration=0.88
//     Overall confidence is below the auto-import threshold; queue entry stays pending
//     until the user manually confirms the match.
//
// Functional: k6 run tests/k6/flows/music-import-flow.js
// Load:       k6 run --vus 10 --duration 60s tests/k6/flows/music-import-flow.js

import http from 'k6/http';
import { check, group } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

// ── helpers ──────────────────────────────────────────────────────────────────

function createArtist(name) {
  const res = http.post(`${BASE_URL}/library-entries`,
    JSON.stringify({ kind: 'artist', contentType: 'music', name }),
    { headers: JSON_HEADERS });
  check(res, { [`create artist ${name} 201`]: r => r.status === 201 });
  return res.json('id');
}

function createGroup(entryId, title, year) {
  const res = http.post(`${BASE_URL}/groups`,
    JSON.stringify({ libraryEntryId: entryId, title, year }),
    { headers: JSON_HEADERS });
  check(res, { [`create group "${title}" 201`]: r => r.status === 201 });
  return res.json('id');
}

// ─────────────────────────────────────────────────────────────────────────────

export default function () {

  // ── FLOW 1: REO Speedwagon — Hi Infidelity ─────────────────────────────
  // Barcode present + ISRCs present → high confidence → auto-import path.
  // Queue entry is dismissed once import is confirmed.

  group('REO Speedwagon: barcode import path', () => {
    let entryId, groupId, releaseId, queueId;

    group('create artist', () => {
      entryId = createArtist('REO Speedwagon');
    });

    group('create release group', () => {
      groupId = createGroup(entryId, 'Hi Infidelity', 1981);
      const res = http.get(`${BASE_URL}/groups/${groupId}`);
      check(res, {
        'release group retrievable':      r => r.status === 200,
        'release group title correct':    r => r.json('title') === 'Hi Infidelity',
        'release group linked to artist': r => r.json('libraryEntryId') === entryId,
      });
    });

    group('create release stub', () => {
      const res = http.post(`${BASE_URL}/music/releases`,
        JSON.stringify({
          groupId,
          libraryEntryId: entryId,
          title:       'Hi Infidelity',
          country:     'US',
          date:        '1981-01-01',
          barcode:     '074646161425',
          format:      'CD',
          mediumCount: 1,
          trackCount:  10,
          isDefault:   true,
          status:      'stub',
        }),
        { headers: JSON_HEADERS });
      check(res, {
        'release stub created':           r => r.status === 201,
        'release status is stub':         r => r.json('status') === 'stub',
        'release linked to group':        r => r.json('groupId') === groupId,
        'release barcode correct':        r => r.json('barcode') === '074646161425',
        'release trackCount correct':     r => r.json('trackCount') === 10,
        'release mediumCount correct':    r => r.json('mediumCount') === 1,
      });
      releaseId = res.json('id');
    });

    group('list releases by group', () => {
      const res = http.get(`${BASE_URL}/groups/${groupId}/releases`);
      check(res, {
        'releases listed':      r => r.status === 200,
        'exactly 1 release':    r => r.json().length === 1,
        'release id matches':   r => r.json('0.id') === releaseId,
      });
    });

    group('scanner: queue entry with high-confidence signals', () => {
      // Barcode matched immediately + ISRCs confirmed on multiple tracks.
      // Pipeline sets overall confidence well above the auto-import threshold.
      const res = http.post(`${BASE_URL}/music/queue`,
        JSON.stringify({
          folderPath:  '/music/REO Speedwagon/Hi Infidelity',
          totalTracks: 10,
          totalDiscs:  1,
          status:      'pending',
          candidates: [{
            artistName:        'REO Speedwagon',
            releaseGroupTitle: 'Hi Infidelity',
            releaseTitle:      'Hi Infidelity',
            overallConfidence: 0.92,
            signals: {
              barcode:       1.0,   // barcode present and matched
              isrc:          0.95,  // ISRCs confirmed on 9 of 10 tracks
              rgNameFuzzy:   0.62,  // name match (moderate; barcode made it unnecessary)
              trackCount:    0.20,  // track count confirmed
              trackTitleSet: 0.45,  // track titles match
              duration:      0.88,  // per-track duration within ±2s
              acoustid:      0.0,   // not yet computed
            },
          }],
        }),
        { headers: JSON_HEADERS });
      check(res, {
        'queue entry created':              r => r.status === 201,
        'queue status is pending':          r => r.json('status') === 'pending',
        'candidate confidence is 0.92':     r => r.json('candidates.0.overallConfidence') === 0.92,
        'signal barcode is 1.0':            r => r.json('candidates.0.signals.barcode') === 1.0,
        'signal isrc is 0.95':              r => r.json('candidates.0.signals.isrc') === 0.95,
        'signal acoustid is 0.0 not null':  r => r.json('candidates.0.signals.acoustid') === 0.0,
        'all 7 signals present':            r => Object.keys(r.json('candidates.0.signals')).length === 7,
      });
      queueId = res.json('id');
    });

    group('import: mark release imported', () => {
      const res = http.patch(`${BASE_URL}/music/releases/${releaseId}`,
        JSON.stringify({ status: 'imported', monitored: true }),
        { headers: JSON_HEADERS });
      check(res, {
        'patch accepted':        r => r.status === 200,
        'status is imported':    r => r.json('status') === 'imported',
        'monitored is true':     r => r.json('monitored') === true,
      });
    });

    group('dismiss queue entry after import', () => {
      const delRes = http.del(`${BASE_URL}/music/queue/${queueId}`);
      check(delRes, { 'dismiss 204': r => r.status === 204 });

      const getRes = http.get(`${BASE_URL}/music/queue/${queueId}`);
      check(getRes, {
        'dismissed entry retrievable': r => r.status === 200,
        'status is dismissed':         r => r.json('status') === 'dismissed',
      });
    });
  });

  // ── FLOW 2: Stevie Nicks — The Enchanted Works of Stevie Nicks ─────────
  // Solo artist. 3-disc compilation. No barcode, no ISRCs, no MBZ IDs in tags.
  // Pipeline falls back to fuzzy name + track count + track title set + duration.
  // Overall confidence is below the auto-import threshold → queue entry stays
  // pending until the user manually confirms the match.

  group('Stevie Nicks: fuzzy-only 3-disc path', () => {
    let entryId, groupId, releaseId, queueId;

    group('create solo artist', () => {
      entryId = createArtist('Stevie Nicks');
    });

    group('create release group (compilation)', () => {
      groupId = createGroup(entryId, 'The Enchanted Works of Stevie Nicks', 1998);
      const res = http.get(`${BASE_URL}/groups/${groupId}`);
      check(res, {
        'release group retrievable':      r => r.status === 200,
        'release group title correct':    r => r.json('title') === 'The Enchanted Works of Stevie Nicks',
        'release group linked to artist': r => r.json('libraryEntryId') === entryId,
      });
    });

    group('create 3-disc release stub (no barcode)', () => {
      // Atlantic catalog 83093-2, US pressing, 46 tracks across 3 discs.
      // Barcode field intentionally absent — this is the worst-case file set.
      const res = http.post(`${BASE_URL}/music/releases`,
        JSON.stringify({
          groupId,
          libraryEntryId: entryId,
          title:       'The Enchanted Works of Stevie Nicks',
          country:     'US',
          date:        '1998-01-01',
          barcode:     '',
          format:      'CD',
          mediumCount: 3,
          trackCount:  46,
          isDefault:   true,
          status:      'stub',
        }),
        { headers: JSON_HEADERS });
      check(res, {
        'release stub created':        r => r.status === 201,
        'release status is stub':      r => r.json('status') === 'stub',
        'release linked to group':     r => r.json('groupId') === groupId,
        'release mediumCount is 3':    r => r.json('mediumCount') === 3,
        'release trackCount is 46':    r => r.json('trackCount') === 46,
        'release barcode is empty':    r => r.json('barcode') === '',
      });
      releaseId = res.json('id');
    });

    group('list releases by group', () => {
      const res = http.get(`${BASE_URL}/groups/${groupId}/releases`);
      check(res, {
        'releases listed':    r => r.status === 200,
        'exactly 1 release':  r => r.json().length === 1,
        'release id matches': r => r.json('0.id') === releaseId,
      });
    });

    group('scanner: queue entry with fuzzy-only signals', () => {
      // No barcode, no ISRCs in the files. Pipeline was forced to use:
      //   - fuzzy album name match against all Stevie Nicks release groups in MBZ
      //   - 46-track + 3-disc count confirmed against the matched RG
      //   - per-track title comparison
      //   - per-track duration (±2s) against MBZ tracklist
      // AcoustID not yet computed. Overall confidence below auto-import threshold.
      const res = http.post(`${BASE_URL}/music/queue`,
        JSON.stringify({
          folderPath:  '/music/Stevie Nicks/The Enchanted Works of Stevie Nicks',
          totalTracks: 46,
          totalDiscs:  3,
          status:      'pending',
          candidates: [{
            artistName:        'Stevie Nicks',
            releaseGroupTitle: 'The Enchanted Works of Stevie Nicks',
            releaseTitle:      'The Enchanted Works of Stevie Nicks',
            overallConfidence: 0.58,
            signals: {
              barcode:       0.0,   // no barcode in files
              isrc:          0.0,   // no ISRCs in files
              rgNameFuzzy:   0.62,  // exact name match from MBZ fuzzy search
              trackCount:    0.20,  // 46 tracks across 3 discs confirmed
              trackTitleSet: 0.45,  // track titles match MBZ tracklist
              duration:      0.88,  // per-track duration within ±2s of MBZ data
              acoustid:      0.0,   // not yet computed
            },
          }],
        }),
        { headers: JSON_HEADERS });
      check(res, {
        'queue entry created':                  r => r.status === 201,
        'queue status is pending':              r => r.json('status') === 'pending',
        'totalTracks is 46':                    r => r.json('totalTracks') === 46,
        'totalDiscs is 3':                      r => r.json('totalDiscs') === 3,
        'overall confidence is 0.58':           r => r.json('candidates.0.overallConfidence') === 0.58,
        'signal barcode is 0.0 not null':       r => r.json('candidates.0.signals.barcode') === 0.0,
        'signal isrc is 0.0 not null':          r => r.json('candidates.0.signals.isrc') === 0.0,
        'signal rgNameFuzzy is 0.62':           r => r.json('candidates.0.signals.rgNameFuzzy') === 0.62,
        'signal trackCount is 0.20':            r => r.json('candidates.0.signals.trackCount') === 0.20,
        'signal duration is 0.88':              r => r.json('candidates.0.signals.duration') === 0.88,
        'all 7 signals present (no omitempty)': r => Object.keys(r.json('candidates.0.signals')).length === 7,
      });
      queueId = res.json('id');
    });

    group('queue stays pending (below auto-import threshold)', () => {
      // Confidence 0.58 < threshold → no auto-import; user must confirm.
      const res = http.get(`${BASE_URL}/music/queue/${queueId}`);
      check(res, {
        'queue entry is still pending': r => r.json('status') === 'pending',
        'still has 1 candidate':        r => r.json('candidates').length === 1,
      });

      const listRes = http.get(`${BASE_URL}/music/queue?status=pending`);
      check(listRes, {
        'pending queue is non-empty': r => r.json().length >= 1,
      });
    });
  });
}
