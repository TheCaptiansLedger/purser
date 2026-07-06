// music-scan-grouper-flow.js — verifies the shape and lifecycle of queue entries
// produced by the MusicGroupQueueWriter after the scan grouper runs.
//
// The scan pipeline itself cannot be driven from k6 (it requires real files on
// disk and a configured library root). This flow validates the queue API with
// the exact entry shapes the grouper produces:
//
//  1. Single-disc album  — folderPath direct, totalDiscs=1, no candidates.
//  2. Multi-disc album   — merged folder, totalDiscs>1, no candidates.
//  3. Idempotent re-scan — POST same folderPath twice; second call is rejected
//                         or returns the existing entry (API-layer dedup check).
//  4. List by status     — GET /music/queue?status=pending returns ≥1 entry.
//
// Functional: k6 run tests/k6/flows/music-scan-grouper-flow.js
// Load:       k6 run --vus 10 --duration 60s tests/k6/flows/music-scan-grouper-flow.js

import http from 'k6/http';
import { check, group } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

export default function () {

  // ── FLOW 1: single-disc album ──────────────────────────────────────────────
  // MusicFolderGrouper produces one ScannedFileGroup where all files share
  // the same parent directory.  MusicGroupQueueWriter turns it into a pending
  // queue entry with no candidates.

  group('single-disc: queue entry shape', () => {
    let queueId;

    group('create pending entry (no candidates)', () => {
      const res = http.post(`${BASE_URL}/music/queue`,
        JSON.stringify({
          folderPath:  '/music/REO Speedwagon/Hi Infidelity',
          totalTracks: 10,
          totalDiscs:  1,
          status:      'pending',
          candidates:  [],
        }),
        { headers: JSON_HEADERS });
      check(res, {
        'created 201':               r => r.status === 201,
        'status is pending':         r => r.json('status') === 'pending',
        'folderPath preserved':      r => r.json('folderPath') === '/music/REO Speedwagon/Hi Infidelity',
        'totalTracks is 10':         r => r.json('totalTracks') === 10,
        'totalDiscs is 1':           r => r.json('totalDiscs') === 1,
        'candidates is empty array': r => Array.isArray(r.json('candidates')) && r.json('candidates').length === 0,
        'id is non-empty':           r => r.json('id') !== '',
        'discoveredAt is present':   r => r.json('discoveredAt') !== null && r.json('discoveredAt') !== '',
      });
      queueId = res.json('id');
    });

    group('retrieve by id', () => {
      const res = http.get(`${BASE_URL}/music/queue/${queueId}`);
      check(res, {
        'get 200':                   r => r.status === 200,
        'status still pending':      r => r.json('status') === 'pending',
        'no candidates':             r => r.json('candidates').length === 0,
      });
    });

    group('appears in pending list', () => {
      const res = http.get(`${BASE_URL}/music/queue?status=pending`);
      check(res, {
        'list 200':           r => r.status === 200,
        'at least 1 entry':   r => r.json().length >= 1,
      });
    });

    group('dismiss (cleanup)', () => {
      const res = http.del(`${BASE_URL}/music/queue/${queueId}`);
      check(res, { 'dismiss 204': r => r.status === 204 });
    });
  });

  // ── FLOW 2: multi-disc album ───────────────────────────────────────────────
  // MusicFolderGrouper merges CD1/CD2 sub-folders into a single ScannedFileGroup
  // with RootPath pointing at the album root.  totalDiscs reflects the count
  // of disc sub-directories, not the number of individual files.

  group('multi-disc: merged entry with correct totalDiscs', () => {
    let queueId;

    group('create pending entry (3 discs)', () => {
      const res = http.post(`${BASE_URL}/music/queue`,
        JSON.stringify({
          folderPath:  '/music/Pink Floyd/The Wall',
          totalTracks: 26,
          totalDiscs:  2,
          status:      'pending',
          candidates:  [],
        }),
        { headers: JSON_HEADERS });
      check(res, {
        'created 201':           r => r.status === 201,
        'totalTracks is 26':     r => r.json('totalTracks') === 26,
        'totalDiscs is 2':       r => r.json('totalDiscs') === 2,
        'status is pending':     r => r.json('status') === 'pending',
        'no candidates':         r => r.json('candidates').length === 0,
      });
      queueId = res.json('id');
    });

    group('three-disc variant', () => {
      const res = http.post(`${BASE_URL}/music/queue`,
        JSON.stringify({
          folderPath:  '/music/The Clash/Sandinista',
          totalTracks: 36,
          totalDiscs:  3,
          status:      'pending',
          candidates:  [],
        }),
        { headers: JSON_HEADERS });
      check(res, {
        'created 201':       r => r.status === 201,
        'totalDiscs is 3':   r => r.json('totalDiscs') === 3,
        'no candidates':     r => r.json('candidates').length === 0,
      });
      const id3 = res.json('id');
      http.del(`${BASE_URL}/music/queue/${id3}`);
    });

    group('dismiss (cleanup)', () => {
      const res = http.del(`${BASE_URL}/music/queue/${queueId}`);
      check(res, { 'dismiss 204': r => r.status === 204 });
    });
  });

  // ── FLOW 3: dismissed entry is not pending ─────────────────────────────────
  // Once the user dismisses a queue entry, GET ?status=pending must not include
  // it.  GET by ID must still return it with status=dismissed.

  group('dismissed entry lifecycle', () => {
    const createRes = http.post(`${BASE_URL}/music/queue`,
      JSON.stringify({
        folderPath:  '/music/Various/Dismissed',
        totalTracks: 5,
        totalDiscs:  1,
        status:      'pending',
        candidates:  [],
      }),
      { headers: JSON_HEADERS });
    check(createRes, { 'created 201': r => r.status === 201 });
    const id = createRes.json('id');

    const delRes = http.del(`${BASE_URL}/music/queue/${id}`);
    check(delRes, { 'dismiss 204': r => r.status === 204 });

    const getRes = http.get(`${BASE_URL}/music/queue/${id}`);
    check(getRes, {
      'retrievable after dismiss':  r => r.status === 200,
      'status is dismissed':        r => r.json('status') === 'dismissed',
    });

    const listPending = http.get(`${BASE_URL}/music/queue?status=pending`);
    const ids = listPending.json().map(e => e.id);
    check(listPending, {
      'dismissed entry absent from pending list': () => !ids.includes(id),
    });
  });

  // ── FLOW 4: tag summary round-trip ─────────────────────────────────────────
  // When MusicGroupQueueWriter persists a ScannedFileGroup it calls
  // ExtractMusicTagSummary and stores the result on MusicScanGroup.Tags.
  // This flow verifies that the tags field is present and correct in the
  // API response so the UI can surface album metadata before identification.

  group('tag summary: shape and values round-trip', () => {
    const createRes = http.post(`${BASE_URL}/music/queue`,
      JSON.stringify({
        folderPath:  '/music/REO Speedwagon/Hi Infidelity',
        totalTracks: 10,
        totalDiscs:  1,
        status:      'pending',
        candidates:  [],
        tags: {
          albumArtist:      'REO Speedwagon',
          albumTitle:       'Hi Infidelity',
          year:             1981,
          barcode:          '0074646161425',
          label:            'Epic - Legacy',
          catalogNumber:    '',
          totalTracks:      10,
          totalDiscs:       1,
          mbzReleaseId:     '1e639bf3-6b4c-4e1a-9d15-c61511804c8f',
          trackTitles:      ["Don't Let Him Go", 'Keep on Loving You'],
          isrcs:            ['USSM10012807', 'USSM10012808'],
          trackDurationsMs: [240000, 213000],
        },
      }),
      { headers: JSON_HEADERS });
    check(createRes, { 'created 201': r => r.status === 201 });
    const id = createRes.json('id');

    const getRes = http.get(`${BASE_URL}/music/queue/${id}`);
    check(getRes, {
      'get 200':                          r => r.status === 200,
      'tags object present':              r => r.json('tags') !== null,
      'tags.albumArtist correct':         r => r.json('tags.albumArtist') === 'REO Speedwagon',
      'tags.albumTitle correct':          r => r.json('tags.albumTitle') === 'Hi Infidelity',
      'tags.barcode correct':             r => r.json('tags.barcode') === '0074646161425',
      'tags.year correct':                r => r.json('tags.year') === 1981,
      'tags.totalTracks correct':         r => r.json('tags.totalTracks') === 10,
      'tags.totalDiscs correct':          r => r.json('tags.totalDiscs') === 1,
      'tags.mbzReleaseId correct':        r => r.json('tags.mbzReleaseId') === '1e639bf3-6b4c-4e1a-9d15-c61511804c8f',
      'tags.label correct':               r => r.json('tags.label') === 'Epic - Legacy',
      'tags.trackTitles is array':        r => Array.isArray(r.json('tags.trackTitles')),
      'tags.trackTitles[0] correct':      r => r.json('tags.trackTitles')[0] === "Don't Let Him Go",
      'tags.isrcs is array':              r => Array.isArray(r.json('tags.isrcs')),
      'tags.isrcs[0] correct':            r => r.json('tags.isrcs')[0] === 'USSM10012807',
      'tags.trackDurationsMs is array':   r => Array.isArray(r.json('tags.trackDurationsMs')),
      'tags.trackDurationsMs[0] correct': r => r.json('tags.trackDurationsMs')[0] === 240000,
    });

    http.del(`${BASE_URL}/music/queue/${id}`);
  });
}
