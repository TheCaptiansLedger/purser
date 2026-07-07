// Endpoint tests for music scan grouper RG signal round-trip
// Functional: k6 run tests/k6/endpoints/music-scan-grouper-endpoints.js
// Load:       k6 run --vus 20 --duration 30s tests/k6/endpoints/music-scan-grouper-endpoints.js

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

const RG_SIGNALS = {
  barcode:       0.0,
  isrc:          0.0,
  rgNameFuzzy:   0.75,
  trackCount:    0.20,
  trackTitleSet: 0.60,
  duration:      0.0,
  acoustid:      0.0,
};

export default function () {
  // POST — create queue entry with all seven RG signals
  const createRes = http.post(`${BASE_URL}/music/queue`,
    JSON.stringify({
      folderPath:  '/music/rg-signals-check',
      totalTracks: 10,
      totalDiscs:  1,
      status:      'pending',
      candidates:  [{
        artistName:        'REO Speedwagon',
        releaseGroupTitle: 'Hi Infidelity',
        releaseTitle:      'Hi Infidelity',
        overallConfidence: 0.55,
        signals:           RG_SIGNALS,
      }],
    }),
    { headers: JSON_HEADERS });
  check(createRes, {
    'POST /music/queue 201':      r => r.status === 201,
    'create: id present':         r => r.json('id') !== '',
    'create: 1 candidate':        r => r.json('candidates').length === 1,
  });
  const queueId = createRes.json('id');

  // GET — read back and assert all seven signal fields
  const getRes = http.get(`${BASE_URL}/music/queue/${queueId}`);
  const signals = getRes.json('candidates.0.signals');
  check(getRes, {
    'GET /music/queue/{id} 200':          r => r.status === 200,
    'signals: 7 keys':                    () => Object.keys(signals).length === 7,
    'signals.rgNameFuzzy >= 0.50':        () => signals.rgNameFuzzy >= 0.50,
    'signals.barcode === 0.0 not null':   () => signals.barcode === 0.0,
    'signals.isrc === 0.0 not null':      () => signals.isrc === 0.0,
    'signals.trackCount === 0.20':        () => signals.trackCount === 0.20,
    'signals.trackTitleSet === 0.60':     () => signals.trackTitleSet === 0.60,
    'signals.duration === 0.0 not null':  () => signals.duration === 0.0,
    'signals.acoustid === 0.0 not null':  () => signals.acoustid === 0.0,
    'no null signal values':              () => Object.values(signals).every(v => v !== null),
  });

  // DELETE — cleanup
  const delRes = http.del(`${BASE_URL}/music/queue/${queueId}`);
  check(delRes, { 'DELETE /music/queue/{id} 204': r => r.status === 204 });

  // POST — non-zero duration signal round-trip (release signal score)
  const durCreateRes = http.post(`${BASE_URL}/music/queue`,
    JSON.stringify({
      folderPath:  '/music/release-signal-duration-check',
      totalTracks: 10,
      totalDiscs:  1,
      status:      'pending',
      candidates:  [{
        artistName:        'REO Speedwagon',
        releaseGroupTitle: 'Hi Infidelity',
        releaseTitle:      'Hi Infidelity (Original)',
        overallConfidence: 0.82,
        signals: {
          barcode:       0.0,
          isrc:          0.0,
          rgNameFuzzy:   0.62,
          trackCount:    0.20,
          trackTitleSet: 0.45,
          duration:      0.95,
          acoustid:      0.0,
        },
      }],
    }),
    { headers: JSON_HEADERS });
  check(durCreateRes, {
    'POST /music/queue with duration=0.95: 201': r => r.status === 201,
    'create duration: id present':              r => r.json('id') !== '',
  });
  const durQueueId = durCreateRes.json('id');

  // GET — assert non-zero duration round-trips as number
  const durGetRes = http.get(`${BASE_URL}/music/queue/${durQueueId}`);
  const durSignals = durGetRes.json('candidates.0.signals');
  check(durGetRes, {
    'GET duration round-trip 200':              r => r.status === 200,
    'signals.duration >= 0.90':                () => durSignals.duration >= 0.90,
    'signals.duration is a number not null':   () => typeof durSignals.duration === 'number',
    'signals.duration not string':             () => typeof durSignals.duration !== 'string',
  });

  // DELETE — cleanup
  const durDelRes = http.del(`${BASE_URL}/music/queue/${durQueueId}`);
  check(durDelRes, { 'DELETE duration entry 204': r => r.status === 204 });
}
