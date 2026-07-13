// k6 gRPC suite for TagAssignmentService — a join-shaped, polymorphic
// entity keyed by the composite (tagId, entityType, entityId). No Update:
// nothing about a TagAssignment is mutable. See
// test/k6/grpc/external_id_test.js for the entityType convention this
// follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/tag_assignment.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const tagId = `k6-grpc-tag-${__VU}-${__ITER}-${Date.now()}`;
  const entityId = 'k6-person-1';
  const entityType = 'ENTITY_TYPE_PERSON';

  let res = client.invoke('purser.domain.v1.TagAssignmentService/CreateTagAssignment', {
    tagAssignment: { tagId: tagId, entityType: entityType, entityId: entityId },
  });
  check(res, {
    'CreateTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateTagAssignment returns the tagId': (r) => r && r.message && r.message.tagAssignment && r.message.tagAssignment.tagId === tagId,
  });

  res = client.invoke('purser.domain.v1.TagAssignmentService/GetTagAssignment', { tagId: tagId, entityType: entityType, entityId: entityId });
  check(res, {
    'GetTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetTagAssignment returns the created entityId': (r) => r && r.message && r.message.tagAssignment && r.message.tagAssignment.entityId === entityId,
  });

  res = client.invoke('purser.domain.v1.TagAssignmentService/ListTagAssignments', { tagId: tagId, pageSize: 10 });
  check(res, {
    'ListTagAssignments filtered by tagId status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListTagAssignments filtered by tagId includes the created assignment': (r) =>
      r && r.message && r.message.tagAssignments && r.message.tagAssignments.some((ta) => ta.entityId === entityId),
  });

  res = client.invoke('purser.domain.v1.TagAssignmentService/ListTagAssignments', { entityType: entityType, entityId: entityId, pageSize: 10 });
  check(res, {
    'ListTagAssignments filtered by entity status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListTagAssignments filtered by entity includes the created assignment': (r) =>
      r && r.message && r.message.tagAssignments && r.message.tagAssignments.some((ta) => ta.tagId === tagId),
  });

  res = client.invoke('purser.domain.v1.TagAssignmentService/DeleteTagAssignment', { tagId: tagId, entityType: entityType, entityId: entityId });
  check(res, { 'DeleteTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.TagAssignmentService/GetTagAssignment', { tagId: tagId, entityType: entityType, entityId: entityId });
  check(res, { 'GetTagAssignment after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
