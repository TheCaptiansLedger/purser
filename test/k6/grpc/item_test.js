// k6 gRPC suite for ItemService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/item.proto', 'purser/domain/v1/media_file.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;
  const mediaFileId = `k6-grpc-item-media-${__VU}-${__ITER}-${Date.now()}`;

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

  res = client.invoke('purser.domain.v1.ItemService/ListItems', { libraryEntryId: 'entry1', contentType: 'adult', pageSize: 10 });
  check(res, {
    'ListItems filtered by libraryEntryId+contentType status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListItems filtered by libraryEntryId+contentType includes the created item': (r) =>
      r && r.message && r.message.items && r.message.items.some((i) => i.id === id),
  });

  res = client.invoke('purser.domain.v1.ItemService/ListItems', { libraryEntryId: 'no-such-entry', pageSize: 10 });
  check(res, {
    'ListItems filtered by a non-matching libraryEntryId excludes the created item': (r) =>
      r && r.message && !(r.message.items || []).some((i) => i.id === id),
  });

  res = client.invoke('purser.domain.v1.MediaFileService/CreateMediaFile', { mediaFile: { id: mediaFileId, itemId: id, path: '/media/k6-item-deletion.mkv' } });
  check(res, { 'CreateMediaFile status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemService/GetItemDeletionImpact', { id: id });
  check(res, {
    'GetItemDeletionImpact status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetItemDeletionImpact reports the media file': (r) =>
      r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'media_file' && i.count === 1),
  });

  res = client.invoke('purser.domain.v1.ItemService/DeleteItem', { id: id });
  check(res, { 'DeleteItem status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemService/GetItem', { id: id });
  check(res, { 'GetItem after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = client.invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: mediaFileId });
  check(res, { 'GetMediaFile after Item Delete is NotFound (unlinked)': (r) => r && r.status === grpc.StatusNotFound });

  // BulkDeleteItems: the "delete these 12 duplicate scenes" use case — one
  // of the two entities ADR 0016 names for a real bulk-delete endpoint.
  const bulkId1 = `k6-grpc-bulk-item-${__VU}-${__ITER}-${Date.now()}-1`;
  const bulkId2 = `k6-grpc-bulk-item-${__VU}-${__ITER}-${Date.now()}-2`;
  const bulkId3 = `k6-grpc-bulk-item-${__VU}-${__ITER}-${Date.now()}-3`;

  for (const bulkId of [bulkId1, bulkId2, bulkId3]) {
    res = client.invoke('purser.domain.v1.ItemService/CreateItem', {
      item: { id: bulkId, contentType: 'adult', libraryEntryId: 'entry1', title: 'K6 Bulk Item', status: 'ITEM_STATUS_WANTED' },
    });
    check(res, { 'setup: CreateItem status is OK': (r) => r && r.status === grpc.StatusOK });
  }

  res = client.invoke('purser.domain.v1.ItemService/BulkDeleteItems', { ids: [bulkId1, bulkId2] });
  check(res, { 'BulkDeleteItems status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemService/GetItem', { id: bulkId1 });
  check(res, { 'GetItem for bulkId1 after BulkDeleteItems is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = client.invoke('purser.domain.v1.ItemService/GetItem', { id: bulkId2 });
  check(res, { 'GetItem for bulkId2 after BulkDeleteItems is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = client.invoke('purser.domain.v1.ItemService/GetItem', { id: bulkId3 });
  check(res, { 'GetItem for bulkId3 (not in the batch) still exists': (r) => r && r.status === grpc.StatusOK });

  // All-or-nothing: a batch with one missing id must fail entirely — the
  // still-existing bulkId3 must not be removed either.
  res = client.invoke('purser.domain.v1.ItemService/BulkDeleteItems', { ids: [bulkId3, 'k6-grpc-item-missing'] });
  check(res, { 'BulkDeleteItems with a missing id is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = client.invoke('purser.domain.v1.ItemService/GetItem', { id: bulkId3 });
  check(res, { 'GetItem for bulkId3 after failed batch still exists (rolled back)': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemService/DeleteItem', { id: bulkId3 });
  check(res, { 'cleanup: DeleteItem status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
