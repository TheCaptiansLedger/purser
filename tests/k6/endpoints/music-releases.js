// Endpoint tests for /api/v1/music/releases
// Functional: k6 run tests/k6/endpoints/music-releases.js
// Load:       k6 run --vus 20 --duration 30s tests/k6/endpoints/music-releases.js

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, options, JSON_HEADERS } from '../config.js';

export { options };

function createArtist() {
  const res = http.post(`${BASE_URL}/library-entries`,
    JSON.stringify({ kind: 'artist', contentType: 'music', name: 'Test Artist' }),
    { headers: JSON_HEADERS });
  check(res, { 'create artist 201': r => r.status === 201 });
  return res.json('id');
}

function createGroup(entryId) {
  const res = http.post(`${BASE_URL}/groups`,
    JSON.stringify({ libraryEntryId: entryId, title: 'Test Album', year: 1980 }),
    { headers: JSON_HEADERS });
  check(res, { 'create group 201': r => r.status === 201 });
  return res.json('id');
}

export default function () {
  const entryId = createArtist();
  const groupId = createGroup(entryId);

  // POST — create
  const createRes = http.post(`${BASE_URL}/music/releases`,
    JSON.stringify({
      groupId,
      libraryEntryId: entryId,
      title: 'Hi Infidelity',
      country: 'US',
      date: '1980-01-01',
      barcode: '074646161425',
      format: 'CD',
      mediumCount: 1,
      trackCount: 10,
      isDefault: true,
      status: 'stub',
    }),
    { headers: JSON_HEADERS });
  check(createRes, {
    'POST /music/releases 201':     r => r.status === 201,
    'create: id present':           r => r.json('id') !== '',
    'create: title correct':        r => r.json('title') === 'Hi Infidelity',
    'create: barcode correct':      r => r.json('barcode') === '074646161425',
    'create: status stub':          r => r.json('status') === 'stub',
    'create: mediumCount correct':  r => r.json('mediumCount') === 1,
    'create: trackCount correct':   r => r.json('trackCount') === 10,
    'create: isDefault true':       r => r.json('isDefault') === true,
    'create: externalIds is array': r => Array.isArray(r.json('externalIds')),
  });
  const releaseId = createRes.json('id');

  // GET — read back
  const getRes = http.get(`${BASE_URL}/music/releases/${releaseId}`);
  check(getRes, {
    'GET /music/releases/{id} 200': r => r.status === 200,
    'get: id matches':              r => r.json('id') === releaseId,
    'get: groupId matches':         r => r.json('groupId') === groupId,
    'get: country present':         r => r.json('country') === 'US',
    'get: date present':            r => r.json('date') === '1980-01-01',
  });

  // GET — not found
  const notFoundRes = http.get(`${BASE_URL}/music/releases/no-such-id`);
  check(notFoundRes, { 'GET unknown id 404': r => r.status === 404 });

  // GET — list by group
  const listRes = http.get(`${BASE_URL}/groups/${groupId}/releases`);
  check(listRes, {
    'GET /groups/{id}/releases 200': r => r.status === 200,
    'list: has 1 release':           r => r.json().length === 1,
  });

  // PATCH — status
  const patchStatusRes = http.patch(`${BASE_URL}/music/releases/${releaseId}`,
    JSON.stringify({ status: 'imported' }),
    { headers: JSON_HEADERS });
  check(patchStatusRes, {
    'PATCH status 200':          r => r.status === 200,
    'PATCH status: imported':    r => r.json('status') === 'imported',
  });

  // PATCH — monitored
  const patchMonRes = http.patch(`${BASE_URL}/music/releases/${releaseId}`,
    JSON.stringify({ monitored: true }),
    { headers: JSON_HEADERS });
  check(patchMonRes, {
    'PATCH monitored 200':       r => r.status === 200,
    'PATCH monitored: true':     r => r.json('monitored') === true,
  });

  // DELETE
  const delRes = http.del(`${BASE_URL}/music/releases/${releaseId}`);
  check(delRes, { 'DELETE /music/releases/{id} 204': r => r.status === 204 });

  const afterDelRes = http.get(`${BASE_URL}/music/releases/${releaseId}`);
  check(afterDelRes, { 'GET after delete 404': r => r.status === 404 });
}
