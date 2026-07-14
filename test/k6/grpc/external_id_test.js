// k6 gRPC suite for ExternalIDService — a join-shaped entity keyed by the
// composite (entityType, entityId, source); only value is mutable, so
// UpdateExternalID takes no field mask. See
// test/k6/grpc/entry_person_test.js for the composite-key pattern.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/external_id.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const entityId = `k6-grpc-entity-${__VU}-${__ITER}-${Date.now()}`;
  const source = 'stashdb';

  let res = invoke('purser.domain.v1.ExternalIDService/CreateExternalID', {
    externalId: { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source, value: 'abc123' },
  });
  check(res, {
    'CreateExternalID status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateExternalID returns the value': (r) => r && r.message && r.message.externalId && r.message.externalId.value === 'abc123',
  });

  res = invoke('purser.domain.v1.ExternalIDService/GetExternalID', { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source });
  check(res, {
    'GetExternalID status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetExternalID returns the created value': (r) => r && r.message && r.message.externalId && r.message.externalId.value === 'abc123',
  });

  res = invoke('purser.domain.v1.ExternalIDService/UpdateExternalID', {
    externalId: { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source, value: 'xyz789' },
  });
  check(res, {
    'UpdateExternalID status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateExternalID applied the new value': (r) => r && r.message && r.message.externalId && r.message.externalId.value === 'xyz789',
  });

  res = invoke('purser.domain.v1.ExternalIDService/ListExternalIDs', { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, pageSize: 10 });
  check(res, {
    'ListExternalIDs status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListExternalIDs includes the created id': (r) =>
      r && r.message && r.message.externalIds && r.message.externalIds.some((e) => e.entityId === entityId && e.source === source),
  });

  res = invoke('purser.domain.v1.ExternalIDService/DeleteExternalID', { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source });
  check(res, { 'DeleteExternalID status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.ExternalIDService/GetExternalID', { entityType: 'ENTITY_TYPE_PERSON', entityId: entityId, source: source });
  check(res, { 'GetExternalID after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
