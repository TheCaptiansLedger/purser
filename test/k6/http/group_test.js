// k6 HTTP/JSON suite for GroupService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:8080';
const SERVICE = `${BASE_URL}/purser.domain.v1.GroupService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;

  let res = http.post(
    `${SERVICE}/CreateGroup`,
    JSON.stringify({ group: { id: id, libraryEntryId: 'entry1', title: 'K6 HTTP Group', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, {
    'CreateGroup status is 200': (r) => r.status === 200,
    'CreateGroup returns the id': (r) => r.json('group.id') === id,
  });

  res = http.post(`${SERVICE}/GetGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetGroup status is 200': (r) => r.status === 200,
    'GetGroup returns the created title': (r) => r.json('group.title') === 'K6 HTTP Group',
  });

  res = http.post(`${SERVICE}/UpdateGroup`, JSON.stringify({ group: { id: id, title: 'K6 HTTP Group Updated' }, updateMask: 'title' }), HEADERS);
  check(res, {
    'UpdateGroup status is 200': (r) => r.status === 200,
    'UpdateGroup applied the field-masked title': (r) => r.json('group.title') === 'K6 HTTP Group Updated',
  });

  res = http.post(`${SERVICE}/ListGroups`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListGroups status is 200': (r) => r.status === 200,
    'ListGroups includes the created group': (r) => (r.json('groups') || []).some((g) => g.id === id),
  });

  res = http.post(`${SERVICE}/DeleteGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteGroup status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetGroup after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
