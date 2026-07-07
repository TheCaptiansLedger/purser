// music-album-identifier-flow.js — verifies the queue-entry shapes the album
// identifier (Task 14) produces at the end of the identification cascade.
//
// The scan pipeline itself cannot be driven from k6 (it needs real files on disk
// and a configured library root), so — as with music-scan-grouper-flow.js — this
// flow validates the queue API with the exact entry shapes the album identifier
// writes:
//
//  1. Below-threshold group — a pending entry whose top candidate carries the
//     full seven-signal breakdown, every field a non-null number even when zero.
//  2. Re-scan shortcut / auto-import — an auto-imported folder is written back as
//     a `matched` entry and therefore never appears in the pending queue, so a
//     second scan leaves the pending list unchanged.
//
// Functional: k6 run tests/k6/flows/music-album-identifier-flow.js
// Load:       k6 run --vus 10 --duration 60s tests/k6/flows/music-album-identifier-flow.js

import http from 'k6/http';
import { check, group } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

const SIGNAL_KEYS = ['barcode', 'isrc', 'rgNameFuzzy', 'trackCount', 'trackTitleSet', 'duration', 'acoustid'];

export default function () {

  // ── FLOW 1: below-threshold group lands in queue with all 7 signals ──────────
  // When overall confidence is below the auto-import threshold the album
  // identifier persists the ranked candidates onto the pending entry. Each
  // candidate's signals object must contain all seven fields as numbers so the
  // queue UI can render the full breakdown of why the folder did not auto-import.

  group('album identifier: below-threshold group lands in queue with all 7 signals', () => {
    const signals = {
      barcode:       0.0,
      isrc:          0.0,
      rgNameFuzzy:   0.90,
      trackCount:    1.0,
      trackTitleSet: 0.0,
      duration:      0.0,
      acoustid:      0.0,
    };

    const createRes = http.post(`${BASE_URL}/music/queue`,
      JSON.stringify({
        folderPath:  '/music/album-identifier/below-threshold',
        totalTracks: 30,
        totalDiscs:  1,
        status:      'pending',
        candidates:  [{
          artistName:        'Stevie Nicks',
          releaseGroupTitle: 'The Enchanted Works of Stevie Nicks',
          releaseTitle:      'The Enchanted Works of Stevie Nicks',
          overallConfidence: 0.80,
          signals:           signals,
        }],
      }),
      { headers: JSON_HEADERS });
    check(createRes, { 'created 201': r => r.status === 201 });
    const id = createRes.json('id');

    const getRes = http.get(`${BASE_URL}/music/queue/${id}`);
    const gotSignals = getRes.json('candidates.0.signals');
    check(getRes, {
      'get 200':                          r => r.status === 200,
      'status pending':                   r => r.json('status') === 'pending',
      'has one candidate':                r => r.json('candidates').length === 1,
      'overallConfidence round-trips':    r => r.json('candidates.0.overallConfidence') === 0.80,
      'exactly 7 signal fields':          () => Object.keys(gotSignals).length === 7,
      'all expected signal keys present': () => SIGNAL_KEYS.every(k => k in gotSignals),
      'no null signal values':            () => Object.values(gotSignals).every(v => v !== null),
      'all signal values are numbers':    () => Object.values(gotSignals).every(v => typeof v === 'number'),
      'zero signals stay numeric zero':   () => gotSignals.barcode === 0.0 && gotSignals.duration === 0.0,
      'fired signals preserved':          () => gotSignals.rgNameFuzzy === 0.90 && gotSignals.trackCount === 1.0,
    });

    // Appears in the pending list while unresolved.
    const pending = http.get(`${BASE_URL}/music/queue?status=pending`);
    check(pending, {
      'below-threshold entry is pending': r => r.json().map(e => e.id).includes(id),
    });

    http.del(`${BASE_URL}/music/queue/${id}`);
  });

  // ── FLOW 2: re-scan shortcut — auto-imported folder stays out of pending ─────
  // On a barcode hit (or any confidence ≥ threshold) the album identifier calls
  // the importer and flips the folder's queue entry to `matched`. A matched entry
  // must never surface in ?status=pending, so a re-scan of an already-imported
  // folder leaves the pending queue unchanged.

  group('album identifier: re-scan shortcut — queue stays empty on second scan', () => {
    const barcodeHitSignals = {
      barcode:       1.0,
      isrc:          0.0,
      rgNameFuzzy:   0.90,
      trackCount:    1.0,
      trackTitleSet: 0.0,
      duration:      0.0,
      acoustid:      0.0,
    };

    const createRes = http.post(`${BASE_URL}/music/queue`,
      JSON.stringify({
        folderPath:  '/music/album-identifier/auto-imported',
        totalTracks: 10,
        totalDiscs:  1,
        status:      'matched',
        candidates:  [{
          artistName:        'REO Speedwagon',
          releaseGroupTitle: 'Hi Infidelity',
          releaseTitle:      'Hi Infidelity',
          overallConfidence: 1.0,
          signals:           barcodeHitSignals,
        }],
      }),
      { headers: JSON_HEADERS });
    check(createRes, {
      'created 201':       r => r.status === 201,
      'status is matched': r => r.json('status') === 'matched',
    });
    const id = createRes.json('id');

    // The matched (auto-imported) entry must not appear in the pending queue.
    const pending = http.get(`${BASE_URL}/music/queue?status=pending`);
    check(pending, {
      'pending list 200':                     r => r.status === 200,
      'matched entry absent from pending':     () => !pending.json().map(e => e.id).includes(id),
    });

    // A second scan of the same folder does not add the matched entry to pending.
    const pendingAgain = http.get(`${BASE_URL}/music/queue?status=pending`);
    check(pendingAgain, {
      'matched entry still absent on re-scan': () => !pendingAgain.json().map(e => e.id).includes(id),
    });

    // Still retrievable by id with its matched status and full breakdown.
    const getRes = http.get(`${BASE_URL}/music/queue/${id}`);
    const gotSignals = getRes.json('candidates.0.signals');
    check(getRes, {
      'get 200':                       r => r.status === 200,
      'status still matched':          r => r.json('status') === 'matched',
      'barcode signal is 1.0':         () => gotSignals.barcode === 1.0,
      'all 7 signals present':         () => Object.keys(gotSignals).length === 7,
      'no null signal values':         () => Object.values(gotSignals).every(v => v !== null),
    });

    http.del(`${BASE_URL}/music/queue/${id}`);
  });
}
