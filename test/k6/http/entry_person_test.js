// k6 HTTP/JSON suite for EntryPersonService — a join-shaped entity keyed
// by the composite (libraryEntryId, personId, role). See
// test/k6/http/person_test.js for the general pattern.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.EntryPersonService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const libraryEntryId = `k6-http-entry-${__VU}-${__ITER}-${Date.now()}`;
  const personId = 'k6-person-1';
  const role = 'director';

  let res = invoke(
    `${SERVICE}/CreateEntryPerson`,
    JSON.stringify({ entryPerson: { libraryEntryId: libraryEntryId, personId: personId, role: role, creditedAs: 'K6 Director' } }),
    HEADERS
  );
  check(res, {
    'CreateEntryPerson status is 200': (r) => r.status === 200,
    'CreateEntryPerson returns the role': (r) => r.json('entryPerson.role') === role,
  });

  res = invoke(`${SERVICE}/GetEntryPerson`, JSON.stringify({ libraryEntryId: libraryEntryId, personId: personId, role: role }), HEADERS);
  check(res, {
    'GetEntryPerson status is 200': (r) => r.status === 200,
    'GetEntryPerson returns the created credit': (r) => r.json('entryPerson.creditedAs') === 'K6 Director',
  });

  res = invoke(
    `${SERVICE}/UpdateEntryPerson`,
    JSON.stringify({
      entryPerson: { libraryEntryId: libraryEntryId, personId: personId, role: role, creditedAs: 'K6 Director Updated' },
      updateMask: 'creditedAs',
    }),
    HEADERS
  );
  check(res, {
    'UpdateEntryPerson status is 200': (r) => r.status === 200,
    'UpdateEntryPerson applied the field-masked credit': (r) => r.json('entryPerson.creditedAs') === 'K6 Director Updated',
  });

  res = invoke(`${SERVICE}/ListEntryPeople`, JSON.stringify({ libraryEntryId: libraryEntryId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListEntryPeople status is 200': (r) => r.status === 200,
    'ListEntryPeople includes the created credit': (r) =>
      (r.json('entryPeople') || []).some((ep) => ep.personId === personId && ep.role === role),
  });

  res = invoke(`${SERVICE}/DeleteEntryPerson`, JSON.stringify({ libraryEntryId: libraryEntryId, personId: personId, role: role }), HEADERS);
  check(res, { 'DeleteEntryPerson status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetEntryPerson`, JSON.stringify({ libraryEntryId: libraryEntryId, personId: personId, role: role }), HEADERS);
  check(res, { 'GetEntryPerson after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
