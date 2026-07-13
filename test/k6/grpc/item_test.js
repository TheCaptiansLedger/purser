// k6 gRPC suite for ItemService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/item.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;

  let res = client.invoke('purser.domain.v1.ItemService/CreateItem', {
    item: { id: id, contentType: 'adult', libraryEntryId: 'entry1', title: 'K6 gRPC Item', status: 'ITEM_STATUS_WANTED' },
  });
  check(res, {
    'CreateItem status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateItem returns the id': (r) => r && r.message && r.message.item && r.message.item.id === id,
  });

  res = client.invoke('purser.domain.v1.ItemService/GetItem', { id: id });
  check(res, {
    'GetItem status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetItem returns the created title': (r) => r && r.message && r.message.item && r.message.item.title === 'K6 gRPC Item',
  });

  res = client.invoke('purser.domain.v1.ItemService/UpdateItem', {
    item: { id: id, title: 'K6 gRPC Item Updated' },
    updateMask: 'title',
  });
  check(res, {
    'UpdateItem status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateItem applied the field-masked title': (r) => r && r.message && r.message.item && r.message.item.title === 'K6 gRPC Item Updated',
  });

  res = client.invoke('purser.domain.v1.ItemService/ListItems', { pageSize: 10 });
  check(res, {
    'ListItems status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListItems includes the created item': (r) => r && r.message && r.message.items && r.message.items.some((i) => i.id === id),
  });

  res = client.invoke('purser.domain.v1.ItemService/DeleteItem', { id: id });
  check(res, { 'DeleteItem status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemService/GetItem', { id: id });
  check(res, { 'GetItem after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
