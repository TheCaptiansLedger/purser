// k6 gRPC suite for TagService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/tag.proto', 'purser/domain/v1/tag_assignment.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;
  const entityId = 'k6-tag-deletion-entity-1';

  let res = client.invoke('purser.domain.v1.TagService/CreateTag', {
    tag: { id: id, key: 'genre', value: 'gonzo', scope: 'TAG_SCOPE_METADATA' },
  });
  check(res, {
    'CreateTag status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateTag returns the id': (r) => r && r.message && r.message.tag && r.message.tag.id === id,
  });

  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: id });
  check(res, {
    'GetTag status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetTag returns the created value': (r) => r && r.message && r.message.tag && r.message.tag.value === 'gonzo',
  });

  res = client.invoke('purser.domain.v1.TagService/UpdateTag', {
    tag: { id: id, value: 'action' },
    updateMask: 'value',
  });
  check(res, {
    'UpdateTag status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateTag applied the field-masked value': (r) => r && r.message && r.message.tag && r.message.tag.value === 'action',
  });

  res = client.invoke('purser.domain.v1.TagService/ListTags', { pageSize: 10 });
  check(res, {
    'ListTags status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListTags includes the created tag': (r) => r && r.message && r.message.tags && r.message.tags.some((t) => t.id === id),
  });

  res = client.invoke('purser.domain.v1.TagAssignmentService/CreateTagAssignment', {
    tagAssignment: { tagId: id, entityType: 'ENTITY_TYPE_PERSON', entityId: entityId },
  });
  check(res, { 'CreateTagAssignment status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.TagService/GetTagDeletionImpact', { id: id });
  check(res, {
    'GetTagDeletionImpact status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetTagDeletionImpact reports the tag assignment': (r) =>
      r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'tag_assignment' && i.count === 1),
  });

  res = client.invoke('purser.domain.v1.TagService/DeleteTag', { id: id });
  check(res, { 'DeleteTag status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: id });
  check(res, { 'GetTag after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = client.invoke('purser.domain.v1.TagAssignmentService/GetTagAssignment', { tagId: id, entityType: 'ENTITY_TYPE_PERSON', entityId: entityId });
  check(res, { 'GetTagAssignment after Tag Delete is NotFound (unlinked)': (r) => r && r.status === grpc.StatusNotFound });

  // BulkDeleteTags: the "delete these duplicate tags" use case — one of
  // the two entities ADR 0016 names for a real bulk-delete endpoint.
  const bulkId1 = `k6-grpc-bulk-tag-${__VU}-${__ITER}-${Date.now()}-1`;
  const bulkId2 = `k6-grpc-bulk-tag-${__VU}-${__ITER}-${Date.now()}-2`;
  const bulkId3 = `k6-grpc-bulk-tag-${__VU}-${__ITER}-${Date.now()}-3`;

  for (const bulkId of [bulkId1, bulkId2, bulkId3]) {
    res = client.invoke('purser.domain.v1.TagService/CreateTag', {
      tag: { id: bulkId, key: 'genre', value: 'gonzo', scope: 'TAG_SCOPE_METADATA' },
    });
    check(res, { 'setup: CreateTag status is OK': (r) => r && r.status === grpc.StatusOK });
  }

  res = client.invoke('purser.domain.v1.TagService/BulkDeleteTags', { ids: [bulkId1, bulkId2] });
  check(res, { 'BulkDeleteTags status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: bulkId1 });
  check(res, { 'GetTag for bulkId1 after BulkDeleteTags is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: bulkId2 });
  check(res, { 'GetTag for bulkId2 after BulkDeleteTags is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: bulkId3 });
  check(res, { 'GetTag for bulkId3 (not in the batch) still exists': (r) => r && r.status === grpc.StatusOK });

  // All-or-nothing: a batch with one missing id must fail entirely — the
  // still-existing bulkId3 must not be removed either.
  res = client.invoke('purser.domain.v1.TagService/BulkDeleteTags', { ids: [bulkId3, 'k6-grpc-tag-missing'] });
  check(res, { 'BulkDeleteTags with a missing id is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: bulkId3 });
  check(res, { 'GetTag for bulkId3 after failed batch still exists (rolled back)': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.TagService/DeleteTag', { id: bulkId3 });
  check(res, { 'cleanup: DeleteTag status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
