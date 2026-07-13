// k6 HTTP/JSON suite for LibraryEntryService — Connect's HTTP/JSON
// transport. See test/k6/http/person_test.js for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;

  let res = http.post(
    `${SERVICE}/CreateLibraryEntry`,
    JSON.stringify({ libraryEntry: { id: id, contentType: 'adult', kind: 'studio', name: 'K6 HTTP Studio', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, {
    'CreateLibraryEntry status is 200': (r) => r.status === 200,
    'CreateLibraryEntry returns the id': (r) => r.json('libraryEntry.id') === id,
  });

  res = http.post(`${SERVICE}/GetLibraryEntry`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetLibraryEntry status is 200': (r) => r.status === 200,
    'GetLibraryEntry returns the created name': (r) => r.json('libraryEntry.name') === 'K6 HTTP Studio',
  });

  res = http.post(
    `${SERVICE}/UpdateLibraryEntry`,
    JSON.stringify({ libraryEntry: { id: id, name: 'K6 HTTP Studio Updated' }, updateMask: 'name' }),
    HEADERS
  );
  check(res, {
    'UpdateLibraryEntry status is 200': (r) => r.status === 200,
    'UpdateLibraryEntry applied the field-masked name': (r) => r.json('libraryEntry.name') === 'K6 HTTP Studio Updated',
  });

  res = http.post(`${SERVICE}/ListLibraryEntries`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListLibraryEntries status is 200': (r) => r.status === 200,
    'ListLibraryEntries includes the created entry': (r) => (r.json('libraryEntries') || []).some((e) => e.id === id),
  });

  res = http.post(`${SERVICE}/DeleteLibraryEntry`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteLibraryEntry status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetLibraryEntry`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetLibraryEntry after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
