// k6 HTTP/JSON suite for ItemPersonService — a join-shaped entity keyed by
// the composite (itemId, personId, role). See
// test/k6/http/entry_person_test.js for the same pattern.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:8080';
const SERVICE = `${BASE_URL}/purser.domain.v1.ItemPersonService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const itemId = `k6-http-item-${__VU}-${__ITER}-${Date.now()}`;
  const personId = 'k6-person-1';
  const role = 'performer';

  let res = http.post(
    `${SERVICE}/CreateItemPerson`,
    JSON.stringify({ itemPerson: { itemId: itemId, personId: personId, role: role, creditedAs: 'K6 Performer' } }),
    HEADERS
  );
  check(res, {
    'CreateItemPerson status is 200': (r) => r.status === 200,
    'CreateItemPerson returns the role': (r) => r.json('itemPerson.role') === role,
  });

  res = http.post(`${SERVICE}/GetItemPerson`, JSON.stringify({ itemId: itemId, personId: personId, role: role }), HEADERS);
  check(res, {
    'GetItemPerson status is 200': (r) => r.status === 200,
    'GetItemPerson returns the created credit': (r) => r.json('itemPerson.creditedAs') === 'K6 Performer',
  });

  res = http.post(
    `${SERVICE}/UpdateItemPerson`,
    JSON.stringify({
      itemPerson: { itemId: itemId, personId: personId, role: role, creditedAs: 'K6 Performer Updated' },
      updateMask: 'creditedAs',
    }),
    HEADERS
  );
  check(res, {
    'UpdateItemPerson status is 200': (r) => r.status === 200,
    'UpdateItemPerson applied the field-masked credit': (r) => r.json('itemPerson.creditedAs') === 'K6 Performer Updated',
  });

  res = http.post(`${SERVICE}/ListItemPeople`, JSON.stringify({ itemId: itemId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListItemPeople status is 200': (r) => r.status === 200,
    'ListItemPeople includes the created credit': (r) => (r.json('itemPeople') || []).some((ip) => ip.personId === personId && ip.role === role),
  });

  res = http.post(`${SERVICE}/DeleteItemPerson`, JSON.stringify({ itemId: itemId, personId: personId, role: role }), HEADERS);
  check(res, { 'DeleteItemPerson status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetItemPerson`, JSON.stringify({ itemId: itemId, personId: personId, role: role }), HEADERS);
  check(res, { 'GetItemPerson after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
