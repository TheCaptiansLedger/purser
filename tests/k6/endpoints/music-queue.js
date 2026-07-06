// Endpoint tests for /api/v1/music/queue
// Functional: k6 run tests/k6/endpoints/music-queue.js
// Load:       k6 run --vus 20 --duration 30s tests/k6/endpoints/music-queue.js

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

const FULL_SIGNALS = {
  barcode:       0.0,
  isrc:          0.95,
  rgNameFuzzy:   0.62,
  trackCount:    0.20,
  trackTitleSet: 0.45,
  duration:      0.88,
  acoustid:      0.0,
};

export default function () {
  // POST — create with all seven signals
  const createRes = http.post(`${BASE_URL}/music/queue`,
    JSON.stringify({
      folderPath: '/music/hi-infidelity',
      totalTracks: 10,
      totalDiscs: 1,
      status: 'pending',
      candidates: [{
        artistName:        'REO Speedwagon',
        releaseGroupTitle: 'Hi Infidelity',
        releaseTitle:      'Hi Infidelity (Original)',
        overallConfidence: 0.72,
        signals:           FULL_SIGNALS,
      }],
    }),
    { headers: JSON_HEADERS });
  check(createRes, {
    'POST /music/queue 201':      r => r.status === 201,
    'create: id present':         r => r.json('id') !== '',
    'create: folderPath correct': r => r.json('folderPath') === '/music/hi-infidelity',
    'create: totalTracks correct':r => r.json('totalTracks') === 10,
    'create: status pending':     r => r.json('status') === 'pending',
    'create: 1 candidate':        r => r.json('candidates').length === 1,
  });
  const queueId = createRes.json('id');

  // GET — read back with all seven signals
  const getRes = http.get(`${BASE_URL}/music/queue/${queueId}`);
  const signals = getRes.json('candidates.0.signals');
  check(getRes, {
    'GET /music/queue/{id} 200':         r => r.status === 200,
    'signals.barcode present':           () => signals.barcode !== null && signals.barcode !== undefined,
    'signals.isrc present':              () => signals.isrc !== null && signals.isrc !== undefined,
    'signals.rgNameFuzzy present':       () => signals.rgNameFuzzy !== null && signals.rgNameFuzzy !== undefined,
    'signals.trackCount present':        () => signals.trackCount !== null && signals.trackCount !== undefined,
    'signals.trackTitleSet present':     () => signals.trackTitleSet !== null && signals.trackTitleSet !== undefined,
    'signals.duration present':          () => signals.duration !== null && signals.duration !== undefined,
    'signals.acoustid present':          () => signals.acoustid !== null && signals.acoustid !== undefined,
    'signals: no null values':           () => Object.values(signals).every(v => v !== null),
    'signals.isrc value correct':        () => signals.isrc === 0.95,
    'signals.duration value correct':    () => signals.duration === 0.88,
  });

  // GET list — pending returns 1, matched returns 0
  const pendingRes = http.get(`${BASE_URL}/music/queue?status=pending`);
  check(pendingRes, {
    'GET queue?status=pending 200': r => r.status === 200,
    'pending list has entry':       r => r.json().length >= 1,
  });

  const matchedRes = http.get(`${BASE_URL}/music/queue?status=matched`);
  check(matchedRes, {
    'GET queue?status=matched 200':  r => r.status === 200,
    'matched list is empty':         r => r.json().length === 0,
  });

  // DELETE — dismiss
  const delRes = http.del(`${BASE_URL}/music/queue/${queueId}`);
  check(delRes, { 'DELETE /music/queue/{id} 204': r => r.status === 204 });

  const afterDismissRes = http.get(`${BASE_URL}/music/queue/${queueId}`);
  check(afterDismissRes, {
    'GET after dismiss 200':        r => r.status === 200,
    'status is dismissed':          r => r.json('status') === 'dismissed',
  });
}
