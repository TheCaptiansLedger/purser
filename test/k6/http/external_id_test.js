// k6 HTTP/JSON suite for ExternalIDService — a join-shaped entity keyed by
// the composite (entityType, entityId, source); only value is mutable. See
// test/k6/http/entry_person_test.js for the composite-key pattern.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.ExternalIDService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

export default () => {
  const entityId = `k6-http-entity-${__VU}-${__ITER}-${Date.now()}`;
  const source = 'stashdb';

  let res = http.post(
    `${SERVICE}/CreateExternalID`,
    JSON.stringify({ externalId: { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source, value: 'abc123' } }),
    HEADERS
  );
  check(res, {
    'CreateExternalID status is 200': (r) => r.status === 200,
    'CreateExternalID returns the value': (r) => r.json('externalId.value') === 'abc123',
  });

  res = http.post(`${SERVICE}/GetExternalID`, JSON.stringify({ entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source }), HEADERS);
  check(res, {
    'GetExternalID status is 200': (r) => r.status === 200,
    'GetExternalID returns the created value': (r) => r.json('externalId.value') === 'abc123',
  });

  res = http.post(
    `${SERVICE}/UpdateExternalID`,
    JSON.stringify({ externalId: { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source, value: 'xyz789' } }),
    HEADERS
  );
  check(res, {
    'UpdateExternalID status is 200': (r) => r.status === 200,
    'UpdateExternalID applied the new value': (r) => r.json('externalId.value') === 'xyz789',
  });

  res = http.post(`${SERVICE}/ListExternalIDs`, JSON.stringify({ entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListExternalIDs status is 200': (r) => r.status === 200,
    'ListExternalIDs includes the created id': (r) => (r.json('externalIds') || []).some((e) => e.entityId === entityId && e.source === source),
  });

  res = http.post(`${SERVICE}/DeleteExternalID`, JSON.stringify({ entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source }), HEADERS);
  check(res, { 'DeleteExternalID status is 200': (r) => r.status === 200 });

  res = http.post(`${SERVICE}/GetExternalID`, JSON.stringify({ entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source }), HEADERS);
  check(res, { 'GetExternalID after Delete is 404 (NotFound)': (r) => r.status === 404 });
};
