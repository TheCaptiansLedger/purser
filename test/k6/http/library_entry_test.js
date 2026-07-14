// k6 HTTP/JSON suite for LibraryEntryService — Connect's HTTP/JSON
// transport. See test/k6/http/person_test.js for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const GROUP_SERVICE = `${BASE_URL}/purser.domain.v1.GroupService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;

  let res = invoke(
    `${SERVICE}/CreateLibraryEntry`,
    JSON.stringify({ libraryEntry: { id: id, contentType: 'adult', kind: 'studio', name: 'K6 HTTP Studio', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, {
    'CreateLibraryEntry status is 200': (r) => r.status === 200,
    'CreateLibraryEntry returns the id': (r) => r.json('libraryEntry.id') === id,
  });

  res = invoke(`${SERVICE}/GetLibraryEntry`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetLibraryEntry status is 200': (r) => r.status === 200,
    'GetLibraryEntry returns the created name': (r) => r.json('libraryEntry.name') === 'K6 HTTP Studio',
  });

  res = invoke(
    `${SERVICE}/UpdateLibraryEntry`,
    JSON.stringify({ libraryEntry: { id: id, name: 'K6 HTTP Studio Updated' }, updateMask: 'name' }),
    HEADERS
  );
  check(res, {
    'UpdateLibraryEntry status is 200': (r) => r.status === 200,
    'UpdateLibraryEntry applied the field-masked name': (r) => r.json('libraryEntry.name') === 'K6 HTTP Studio Updated',
  });

  res = invoke(`${SERVICE}/ListLibraryEntries`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListLibraryEntries status is 200': (r) => r.status === 200,
    'ListLibraryEntries includes the created entry': (r) => (r.json('libraryEntries') || []).some((e) => e.id === id),
  });

  res = invoke(`${SERVICE}/ListLibraryEntries`, JSON.stringify({ kind: 'studio', pageSize: 10 }), HEADERS);
  check(res, {
    'ListLibraryEntries filtered by kind status is 200': (r) => r.status === 200,
    'ListLibraryEntries filtered by kind includes the created entry': (r) => (r.json('libraryEntries') || []).some((e) => e.id === id),
  });

  res = invoke(`${SERVICE}/ListLibraryEntries`, JSON.stringify({ kind: 'network', pageSize: 10 }), HEADERS);
  check(res, {
    'ListLibraryEntries filtered by a non-matching kind excludes the created entry': (r) =>
      !(r.json('libraryEntries') || []).some((e) => e.id === id),
  });

  // Deletion-impact + Unlink: a child LibraryEntry is a non-blocking
  // referrer — deleting the parent without cascade detaches the child
  // (blanks its parentId) rather than deleting it or failing.
  const childId = `k6-http-le-child-${__VU}-${__ITER}-${Date.now()}`;
  res = invoke(
    `${SERVICE}/CreateLibraryEntry`,
    JSON.stringify({ libraryEntry: { id: childId, contentType: 'adult', kind: 'studio', name: 'K6 HTTP Child Studio', parentId: id, monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateLibraryEntry (child) status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetLibraryEntryDeletionImpact`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetLibraryEntryDeletionImpact status is 200': (r) => r.status === 200,
    'GetLibraryEntryDeletionImpact reports the child as non-blocking': (r) =>
      (r.json('impacts') || []).some((i) => i.kind === 'library_entry_child' && i.count === 1 && !i.blocking),
  });

  res = invoke(`${SERVICE}/DeleteLibraryEntry`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteLibraryEntry status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetLibraryEntry`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetLibraryEntry after Delete is 404 (NotFound)': (r) => r.status === 404 });

  // The child must still exist, just detached (parentId cleared) — a
  // non-blocking referrer is unlinked, not deleted, on a plain Delete.
  // Connect's HTTP/JSON transport omits proto3 zero-value fields from the
  // response body, so an empty parentId shows up as undefined, not ''.
  res = invoke(`${SERVICE}/GetLibraryEntry`, JSON.stringify({ id: childId }), HEADERS);
  check(res, {
    'GetLibraryEntry after parent Delete still finds the child (detached, not deleted)': (r) => r.status === 200,
    'GetLibraryEntry after parent Delete shows parentId cleared': (r) => !r.json('libraryEntry.parentId'),
  });

  // Blocking + cascade: a Group is a structural referrer (required FK) —
  // deleting its LibraryEntry without cascade must fail, and only
  // cascade=true removes both.
  const groupId = `k6-http-le-group-${__VU}-${__ITER}-${Date.now()}`;
  res = invoke(
    `${GROUP_SERVICE}/CreateGroup`,
    JSON.stringify({ group: { id: groupId, libraryEntryId: childId, title: 'K6 HTTP LibraryEntry Deletion Group', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateGroup status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetLibraryEntryDeletionImpact`, JSON.stringify({ id: childId }), HEADERS);
  check(res, {
    'GetLibraryEntryDeletionImpact reports the group as blocking': (r) =>
      (r.json('impacts') || []).some((i) => i.kind === 'group' && i.count === 1 && i.blocking),
  });

  res = invoke(`${SERVICE}/DeleteLibraryEntry`, JSON.stringify({ id: childId }), HEADERS);
  check(res, {
    'DeleteLibraryEntry without cascade is 400 (FailedPrecondition) when a Group exists': (r) => r.status === 400,
  });

  res = invoke(`${SERVICE}/DeleteLibraryEntry`, JSON.stringify({ id: childId, cascade: true }), HEADERS);
  check(res, { 'DeleteLibraryEntry with cascade status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetLibraryEntry`, JSON.stringify({ id: childId }), HEADERS);
  check(res, { 'GetLibraryEntry after cascade Delete is 404 (NotFound)': (r) => r.status === 404 });

  res = invoke(`${GROUP_SERVICE}/GetGroup`, JSON.stringify({ id: groupId }), HEADERS);
  check(res, { 'GetGroup after cascade Delete is 404 (NotFound)': (r) => r.status === 404 });
};
