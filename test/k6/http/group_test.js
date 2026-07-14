// k6 HTTP/JSON suite for GroupService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.GroupService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;
  const itemId = `k6-http-group-item-${__VU}-${__ITER}-${Date.now()}`;

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

  res = http.post(
    `${ITEM}/CreateItem`,
    JSON.stringify({ item: { id: itemId, contentType: 'adult', libraryEntryId: 'entry1', groupId: id, title: 'K6 Group Deletion Item', status: 'ITEM_STATUS_WANTED' } }),
    HEADERS
  );
  check(res, { 'CreateItem status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetGroupDeletionImpact`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetGroupDeletionImpact status is 200': (r) => r.status === 200,
    'GetGroupDeletionImpact reports the item': (r) => (r.json('impacts') || []).some((i) => i.kind === 'item' && i.count === 1),
  });

  res = http.post(`${SERVICE}/DeleteGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteGroup status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetGroup after Delete is 404 (NotFound)': (r) => r.status === 404 });

  // The item must still exist, just detached (groupId cleared) — Group
  // deletion detaches Items rather than deleting them.
  res = http.post(`${ITEM}/GetItem`, JSON.stringify({ id: itemId }), HEADERS);
  check(res, {
    'GetItem after Group Delete still finds the item (detached, not deleted)': (r) => r.status === 200,
    // Connect's protojson mapping omits proto3 zero-value fields (an
    // empty string) from the response entirely, so a cleared groupId
    // shows up as undefined here, not "".
    'GetItem after Group Delete shows groupId cleared': (r) => !r.json('item.groupId'),
  });

  res = http.post(`${ITEM}/DeleteItem`, JSON.stringify({ id: itemId }), HEADERS);
  check(res, { 'cleanup: DeleteItem status is 200': (r) => r.status === 200 });
};
