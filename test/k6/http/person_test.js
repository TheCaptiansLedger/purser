// k6 HTTP/JSON suite for PersonService — Connect's HTTP/JSON transport
// (the same generated handler as the gRPC suite, different wire protocol;
// see docs/adr/0011-api-design.md). Run from the repo root via
// `make k6-http` (or `k6 run test/k6/http/person_test.js`) against a
// running `purser serve` (PURSER_HTTP_URL, default http://localhost:7474).
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.PersonService`;
const PERFORMER_PROFILE = `${BASE_URL}/purser.afterdark.v1.PerformerProfileService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  // id is server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  let res = invoke(
    `${SERVICE}/CreatePerson`,
    JSON.stringify({
      person: { name: 'K6 HTTP Person', gender: 'GENDER_UNKNOWN', monitorMode: 'MONITOR_MODE_NONE' },
    }),
    HEADERS
  );
  check(res, {
    'CreatePerson status is 200': (r) => r.status === 200,
    'CreatePerson returns an id': (r) => !!r.json('person.id'),
  });
  const id = res.json('person.id');

  res = invoke(`${SERVICE}/GetPerson`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetPerson status is 200': (r) => r.status === 200,
    'GetPerson returns the created name': (r) => r.json('person.name') === 'K6 HTTP Person',
  });

  res = invoke(
    `${SERVICE}/UpdatePerson`,
    // google.protobuf.FieldMask's JSON mapping is a comma-joined string of
    // field paths, not {paths: [...]} — the latter is the wire/proto-text
    // shape, and sending it as JSON is a real 400 from the server.
    JSON.stringify({
      person: { id: id, name: 'K6 HTTP Person Updated' },
      updateMask: 'name',
    }),
    HEADERS
  );
  check(res, {
    'UpdatePerson status is 200': (r) => r.status === 200,
    'UpdatePerson applied the field-masked name': (r) => r.json('person.name') === 'K6 HTTP Person Updated',
  });

  res = invoke(`${SERVICE}/ListPeople`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListPeople status is 200': (r) => r.status === 200,
    'ListPeople includes the created person': (r) => (r.json('people') || []).some((p) => p.id === id),
  });

  res = invoke(`${SERVICE}/GetPerson`, JSON.stringify({ id: 'missing-' + id }), HEADERS);
  check(res, {
    'GetPerson on a missing id is 404 (NotFound)': (r) => r.status === 404,
  });

  res = invoke(`${PERFORMER_PROFILE}/CreatePerformerProfile`, JSON.stringify({ performerProfile: { personId: id } }), HEADERS);
  check(res, { 'CreatePerformerProfile status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetPersonDeletionImpact`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetPersonDeletionImpact status is 200': (r) => r.status === 200,
    'GetPersonDeletionImpact reports the performer profile': (r) =>
      (r.json('impacts') || []).some((i) => i.kind === 'performer_profile' && i.count === 1),
  });

  res = invoke(`${SERVICE}/DeletePerson`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'DeletePerson status is 200': (r) => r.status === 200,
  });

  res = invoke(`${SERVICE}/GetPerson`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetPerson after Delete is 404 (NotFound)': (r) => r.status === 404,
  });

  res = invoke(`${PERFORMER_PROFILE}/GetPerformerProfile`, JSON.stringify({ personId: id }), HEADERS);
  check(res, { 'GetPerformerProfile after Person Delete is 404 (unlinked)': (r) => r.status === 404 });
};
