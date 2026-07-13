// k6 HTTP/JSON suite for ItemService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.ItemService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const id = `k6-http-${__VU}-${__ITER}-${Date.now()}`;

  let res = http.post(
    `${SERVICE}/CreateItem`,
    JSON.stringify({ item: { id: id, contentType: 'adult', libraryEntryId: 'entry1', title: 'K6 HTTP Item', status: 'ITEM_STATUS_WANTED' } }),
    HEADERS
  );
  check(res, {
    'CreateItem status is 200': (r) => r.status === 200,
    'CreateItem returns the id': (r) => r.json('item.id') === id,
  });

  res = http.post(`${SERVICE}/GetItem`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetItem status is 200': (r) => r.status === 200,
    'GetItem returns the created title': (r) => r.json('item.title') === 'K6 HTTP Item',
  });

  res = http.post(`${SERVICE}/UpdateItem`, JSON.stringify({ item: { id: id, title: 'K6 HTTP Item Updated' }, updateMask: 'title' }), HEADERS);
  check(res, {
    'UpdateItem status is 200': (r) => r.status === 200,
    'UpdateItem applied the field-masked title': (r) => r.json('item.title') === 'K6 HTTP Item Updated',
  });

  res = http.post(`${SERVICE}/ListItems`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListItems status is 200': (r) => r.status === 200,
    'ListItems includes the created item': (r) => (r.json('items') || []).some((i) => i.id === id),
  });

  res = http.post(`${SERVICE}/ListItems`, JSON.stringify({ libraryEntryId: 'entry1', contentType: 'adult', pageSize: 10 }), HEADERS);
  check(res, {
    'ListItems filtered by libraryEntryId+contentType status is 200': (r) => r.status === 200,
    'ListItems filtered by libraryEntryId+contentType includes the created item': (r) => (r.json('items') || []).some((i) => i.id === id),
  });

  res = http.post(`${SERVICE}/ListItems`, JSON.stringify({ libraryEntryId: 'no-such-entry', pageSize: 10 }), HEADERS);
  check(res, {
    'ListItems filtered by a non-matching libraryEntryId excludes the created item': (r) => !(r.json('items') || []).some((i) => i.id === id),
  });

  res = http.post(`${SERVICE}/DeleteItem`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteItem status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetItem`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetItem after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
