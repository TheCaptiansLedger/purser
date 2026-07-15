// k6 gRPC suite for TagAssignmentService — a join-shaped, polymorphic
// entity keyed by the composite (tagId, entityType, entityId). No Update:
// nothing about a TagAssignment is mutable. See
// test/k6/grpc/external_id_test.js for the entityType convention this
// follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/tag_assignment.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const tagId = `k6-grpc-tag-${__VU}-${__ITER}-${Date.now()}`;
  const entityId = 'k6-person-1';
  const entityType = 'ENTITY_TYPE_PERSON';

  let res = invoke('purser.domain.v1.TagAssignmentService/CreateTagAssignment', {
    tagAssignment: { tagId: tagId, entityType: entityType, entityId: entityId },
  });
  check(res, {
    'CreateTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateTagAssignment returns the tagId': (r) => r && r.message && r.message.tagAssignment && r.message.tagAssignment.tagId === tagId,
  });

  res = invoke('purser.domain.v1.TagAssignmentService/GetTagAssignment', { tagId: tagId, entityType: entityType, entityId: entityId });
  check(res, {
    'GetTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetTagAssignment returns the created entityId': (r) => r && r.message && r.message.tagAssignment && r.message.tagAssignment.entityId === entityId,
  });

  res = invoke('purser.domain.v1.TagAssignmentService/ListTagAssignments', { tagId: tagId, pageSize: 10 });
  check(res, {
    'ListTagAssignments filtered by tagId status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListTagAssignments filtered by tagId includes the created assignment': (r) =>
      r && r.message && r.message.tagAssignments && r.message.tagAssignments.some((ta) => ta.entityId === entityId),
  });

  res = invoke('purser.domain.v1.TagAssignmentService/ListTagAssignments', { entityType: entityType, entityId: entityId, pageSize: 10 });
  check(res, {
    'ListTagAssignments filtered by entity status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListTagAssignments filtered by entity includes the created assignment': (r) =>
      r && r.message && r.message.tagAssignments && r.message.tagAssignments.some((ta) => ta.tagId === tagId),
  });

  res = invoke('purser.domain.v1.TagAssignmentService/DeleteTagAssignment', { tagId: tagId, entityType: entityType, entityId: entityId });
  check(res, { 'DeleteTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.TagAssignmentService/GetTagAssignment', { tagId: tagId, entityType: entityType, entityId: entityId });
  check(res, { 'GetTagAssignment after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // BulkCreateTagAssignments: the "tag these 50 scenes" use case — one Tag
  // attached to many entities of the same type in a single atomic call.
  const bulkTagId = `k6-grpc-bulk-tag-${__VU}-${__ITER}-${Date.now()}`;
  const bulkEntityIds = ['k6-bulk-scene-1', 'k6-bulk-scene-2', 'k6-bulk-scene-3'];

  res = invoke('purser.domain.v1.TagAssignmentService/BulkCreateTagAssignments', {
    tagId: bulkTagId,
    entityType: 'ENTITY_TYPE_ITEM',
    entityIds: bulkEntityIds,
  });
  check(res, {
    'BulkCreateTagAssignments status is OK': (r) => r && r.status === grpc.StatusOK,
    'BulkCreateTagAssignments returns every created assignment': (r) =>
      r && r.message && r.message.tagAssignments && r.message.tagAssignments.length === 3,
  });

  res = invoke('purser.domain.v1.TagAssignmentService/ListTagAssignments', { tagId: bulkTagId, pageSize: 10 });
  check(res, {
    'ListTagAssignments after BulkCreate finds every entity': (r) =>
      r &&
      r.message &&
      r.message.tagAssignments &&
      bulkEntityIds.every((eid) => r.message.tagAssignments.some((ta) => ta.entityId === eid)),
  });

  // All-or-nothing: a batch where one row already exists must fail
  // entirely — the two brand-new rows in the same batch must not be
  // created either.
  res = invoke('purser.domain.v1.TagAssignmentService/BulkCreateTagAssignments', {
    tagId: bulkTagId,
    entityType: 'ENTITY_TYPE_ITEM',
    entityIds: ['k6-bulk-scene-4', 'k6-bulk-scene-1', 'k6-bulk-scene-5'], // scene-1 already exists
  });
  check(res, { 'BulkCreateTagAssignments with a conflicting row is AlreadyExists': (r) => r && r.status === grpc.StatusAlreadyExists });

  res = invoke('purser.domain.v1.TagAssignmentService/GetTagAssignment', { tagId: bulkTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: 'k6-bulk-scene-4' });
  check(res, { 'GetTagAssignment for scene-4 after failed batch is NotFound (rolled back)': (r) => r && r.status === grpc.StatusNotFound });

  // cleanup: no bulk-delete for TagAssignment (0016 scopes bulk delete to
  // Item/Tag only) — delete each row individually.
  for (const eid of bulkEntityIds) {
    res = invoke('purser.domain.v1.TagAssignmentService/DeleteTagAssignment', { tagId: bulkTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: eid });
    check(res, { 'cleanup: DeleteTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK });
  }

  client.close();
};
