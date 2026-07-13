// k6 gRPC suite for GroupService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/group.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;

  let res = client.invoke('purser.domain.v1.GroupService/CreateGroup', {
    group: { id: id, libraryEntryId: 'entry1', title: 'K6 gRPC Group', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, {
    'CreateGroup status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateGroup returns the id': (r) => r && r.message && r.message.group && r.message.group.id === id,
  });

  res = client.invoke('purser.domain.v1.GroupService/GetGroup', { id: id });
  check(res, {
    'GetGroup status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetGroup returns the created title': (r) => r && r.message && r.message.group && r.message.group.title === 'K6 gRPC Group',
  });

  res = client.invoke('purser.domain.v1.GroupService/UpdateGroup', {
    group: { id: id, title: 'K6 gRPC Group Updated' },
    updateMask: 'title',
  });
  check(res, {
    'UpdateGroup status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateGroup applied the field-masked title': (r) => r && r.message && r.message.group && r.message.group.title === 'K6 gRPC Group Updated',
  });

  res = client.invoke('purser.domain.v1.GroupService/ListGroups', { pageSize: 10 });
  check(res, {
    'ListGroups status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListGroups includes the created group': (r) => r && r.message && r.message.groups && r.message.groups.some((g) => g.id === id),
  });

  res = client.invoke('purser.domain.v1.GroupService/DeleteGroup', { id: id });
  check(res, { 'DeleteGroup status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.GroupService/GetGroup', { id: id });
  check(res, { 'GetGroup after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
