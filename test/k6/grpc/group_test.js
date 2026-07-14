// k6 gRPC suite for GroupService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/group.proto', 'purser/domain/v1/item.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  let res = invoke('purser.domain.v1.GroupService/CreateGroup', {
    group: { libraryEntryId: 'entry1', title: 'K6 gRPC Group', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, {
    'CreateGroup status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateGroup returns an id': (r) => r && r.message && r.message.group && !!r.message.group.id,
  });
  const id = res.message.group.id;

  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: id });
  check(res, {
    'GetGroup status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetGroup returns the created title': (r) => r && r.message && r.message.group && r.message.group.title === 'K6 gRPC Group',
  });

  res = invoke('purser.domain.v1.GroupService/UpdateGroup', {
    group: { id: id, title: 'K6 gRPC Group Updated' },
    updateMask: 'title',
  });
  check(res, {
    'UpdateGroup status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateGroup applied the field-masked title': (r) => r && r.message && r.message.group && r.message.group.title === 'K6 gRPC Group Updated',
  });

  res = invoke('purser.domain.v1.GroupService/ListGroups', { pageSize: 10 });
  check(res, {
    'ListGroups status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListGroups includes the created group': (r) => r && r.message && r.message.groups && r.message.groups.some((g) => g.id === id),
  });

  res = invoke('purser.domain.v1.ItemService/CreateItem', {
    item: { contentType: 'adult', libraryEntryId: 'entry1', groupId: id, title: 'K6 Group Deletion Item', status: 'ITEM_STATUS_WANTED' },
  });
  check(res, {
    'CreateItem status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateItem returns an id': (r) => r && r.message && r.message.item && !!r.message.item.id,
  });
  const itemId = res.message.item.id;

  res = invoke('purser.domain.v1.GroupService/GetGroupDeletionImpact', { id: id });
  check(res, {
    'GetGroupDeletionImpact status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetGroupDeletionImpact reports the item': (r) => r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'item' && i.count === 1),
  });

  res = invoke('purser.domain.v1.GroupService/DeleteGroup', { id: id });
  check(res, { 'DeleteGroup status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: id });
  check(res, { 'GetGroup after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // The item must still exist, just detached (groupId cleared) — Group
  // deletion detaches Items rather than deleting them.
  res = invoke('purser.domain.v1.ItemService/GetItem', { id: itemId });
  check(res, {
    'GetItem after Group Delete still finds the item (detached, not deleted)': (r) => r && r.status === grpc.StatusOK,
    'GetItem after Group Delete shows groupId cleared': (r) => r && r.message && r.message.item && r.message.item.groupId === '',
  });

  res = invoke('purser.domain.v1.ItemService/DeleteItem', { id: itemId });
  check(res, { 'cleanup: DeleteItem status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
