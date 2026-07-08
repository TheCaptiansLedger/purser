// Endpoint tests for artist import enrichment (Task 15 / issue #407).
// Verifies: POST /metadata/entries/import for music artists, and
// GET /music/releases/{id}/tracks stub endpoint.
//
// Functional: k6 run tests/k6/endpoints/music-artist-import-endpoints.js
// Load:       k6 run --vus 10 --duration 30s tests/k6/endpoints/music-artist-import-endpoints.js

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

export default function () {
  // POST /api/v1/library-entries — create a minimal artist entry to test against
  const entryRes = http.post(`${BASE_URL}/library-entries`,
    JSON.stringify({ kind: 'artist', contentType: 'music', name: 'Test Import Artist' }),
    { headers: JSON_HEADERS });
  check(entryRes, { 'create artist entry 201': r => r.status === 201 });
  const entryId = entryRes.json('id');

  // Create a group and release so we can test the tracks stub endpoint
  const groupRes = http.post(`${BASE_URL}/groups`,
    JSON.stringify({ libraryEntryId: entryId, title: 'Test Album', year: 2000 }),
    { headers: JSON_HEADERS });
  check(groupRes, { 'create group 201': r => r.status === 201 });
  const groupId = groupRes.json('id');

  const releaseRes = http.post(`${BASE_URL}/music/releases`,
    JSON.stringify({
      groupId,
      libraryEntryId: entryId,
      title: 'Test Album (US)',
      country: 'US',
      date: '2000-03-01',
      isDefault: true,
      status: 'stub',
    }),
    { headers: JSON_HEADERS });
  check(releaseRes, { 'create release 201': r => r.status === 201 });
  const releaseId = releaseRes.json('id');

  // GET /api/v1/music/releases/{id}/tracks — stub endpoint (Task 15, full impl in Task 18)
  const tracksRes = http.get(`${BASE_URL}/music/releases/${releaseId}/tracks`);
  check(tracksRes, {
    'GET /music/releases/{id}/tracks 200':          r => r.status === 200,
    'tracks response is array':                     r => Array.isArray(r.json()),
    'tracks array is empty (stub — no files yet)':  r => r.json().length === 0,
  });

  // GET /api/v1/music/releases/unknown-id/tracks — 404 for non-existent release
  // NOTE: the tracks stub returns [] for any ID since it doesn't validate existence.
  // This is by spec — Task 18 (#410) will add real track loading with 404 handling.
  const tracksMissingRes = http.get(`${BASE_URL}/music/releases/nonexistent-id/tracks`);
  check(tracksMissingRes, {
    'GET tracks for missing release 200 (stub returns [])': r => r.status === 200,
  });

  // POST /api/v1/metadata/entries/import — import a music artist
  // Uses a well-known MBID that exists in MusicBrainz public API.
  // We verify the import endpoint accepts music content type and returns an entry.
  // Enrichment (release groups, releases, members) is verified by the integration test.
  const importRes = http.post(`${BASE_URL}/metadata/entries/import`,
    JSON.stringify({
      source: 'mbz',
      externalId: 'bdc70372-7e8a-4cb9-8d33-f036b3b7cdc1',
      name: 'REO Speedwagon',
      contentType: 'music',
      monitored: false,
      monitorMode: 'none',
    }),
    { headers: JSON_HEADERS });
  check(importRes, {
    'POST /metadata/entries/import 201':  r => r.status === 201,
    'import: id present':                 r => !!r.json('id'),
    'import: kind is artist':             r => r.json('kind') === 'artist',
    'import: contentType is music':       r => r.json('contentType') === 'music',
    'import: name matches':               r => r.json('name') === 'REO Speedwagon',
  });
}
