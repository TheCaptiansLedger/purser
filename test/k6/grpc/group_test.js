// k6 gRPC suite for GroupService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/group.proto', 'purser/domain/v1/item.proto', 'purser/music/v1/release.proto');

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

  // A MusicRelease under this Group, plus a track pointing at it — proves
  // GroupDeletionService's other referrer: since music.Release.GroupID is
  // required (unlike Item.GroupID), Group deletion must delete the
  // Release outright (not detach it), while unlinking the Release's own
  // tracks rather than deleting them. See
  // docs/adr/0021-music-domain-model.md's "Ripple effects" section.
  res = invoke('purser.music.v1.MusicReleaseService/CreateMusicRelease', {
    musicRelease: { groupId: id, libraryEntryId: 'entry1', title: 'K6 Group Deletion Release', status: 'RELEASE_STATUS_STUB' },
  });
  check(res, {
    'CreateMusicRelease status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateMusicRelease returns an id': (r) => r && r.message && r.message.musicRelease && !!r.message.musicRelease.id,
  });
  const releaseId = res.message.musicRelease.id;

  res = invoke('purser.domain.v1.ItemService/CreateItem', {
    item: {
      contentType: 'music',
      libraryEntryId: 'entry1',
      groupId: id,
      title: 'K6 Group Deletion Track',
      status: 'ITEM_STATUS_IMPORTED',
      metadata: { release_id: releaseId },
    },
  });
  check(res, {
    'CreateItem(track) status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateItem(track) returns an id': (r) => r && r.message && r.message.item && !!r.message.item.id,
  });
  const trackId = res.message.item.id;

  res = invoke('purser.domain.v1.GroupService/GetGroupDeletionImpact', { id: id });
  check(res, {
    'GetGroupDeletionImpact status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetGroupDeletionImpact reports the item': (r) => r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'item' && i.count === 2),
    'GetGroupDeletionImpact reports the music_release': (r) => r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'music_release' && i.count === 1),
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

  // The Release must be gone — Group deletion deletes referencing
  // Releases outright, since GroupID is required and can't be detached.
  res = invoke('purser.music.v1.MusicReleaseService/GetMusicRelease', { id: releaseId });
  check(res, { 'GetMusicRelease after Group Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // The track must still exist, just detached (metadata.release_id
  // cleared) — the Release's own Unlink step runs as part of Group
  // deletion.
  res = invoke('purser.domain.v1.ItemService/GetItem', { id: trackId });
  check(res, {
    'GetItem(track) after Group Delete still finds the track (detached, not deleted)': (r) => r && r.status === grpc.StatusOK,
    'GetItem(track) after Group Delete shows metadata.release_id cleared': (r) => r && r.message && r.message.item && !(r.message.item.metadata && 'release_id' in r.message.item.metadata),
  });

  res = invoke('purser.domain.v1.ItemService/DeleteItem', { id: itemId });
  check(res, { 'cleanup: DeleteItem status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.ItemService/DeleteItem', { id: trackId });
  check(res, { 'cleanup: DeleteItem(track) status is OK': (r) => r && r.status === grpc.StatusOK });

  // BulkDeleteGroups: the "delete these 12 duplicate discography groups"
  // use case — see docs/adr/0016-bulk-operations.md.
  const bulkIds = [];
  for (let i = 0; i < 3; i++) {
    res = invoke('purser.domain.v1.GroupService/CreateGroup', {
      group: { libraryEntryId: 'entry1', title: 'K6 Bulk Group', monitorMode: 'MONITOR_MODE_NONE' },
    });
    check(res, {
      'setup: CreateGroup status is OK': (r) => r && r.status === grpc.StatusOK,
      'setup: CreateGroup returns an id': (r) => r && r.message && r.message.group && !!r.message.group.id,
    });
    bulkIds.push(res.message.group.id);
  }
  const [bulkId1, bulkId2, bulkId3] = bulkIds;

  res = invoke('purser.domain.v1.GroupService/BulkDeleteGroups', { ids: [bulkId1, bulkId2] });
  check(res, { 'BulkDeleteGroups status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: bulkId1 });
  check(res, { 'GetGroup for bulkId1 after BulkDeleteGroups is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: bulkId2 });
  check(res, { 'GetGroup for bulkId2 after BulkDeleteGroups is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: bulkId3 });
  check(res, { 'GetGroup for bulkId3 (not in the batch) still exists': (r) => r && r.status === grpc.StatusOK });

  // All-or-nothing: a batch with one missing id must fail entirely — the
  // still-existing bulkId3 must not be removed either.
  res = invoke('purser.domain.v1.GroupService/BulkDeleteGroups', { ids: [bulkId3, 'k6-grpc-group-missing'] });
  check(res, { 'BulkDeleteGroups with a missing id is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: bulkId3 });
  check(res, { 'GetGroup for bulkId3 after failed batch still exists (rolled back)': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.GroupService/DeleteGroup', { id: bulkId3 });
  check(res, { 'cleanup: DeleteGroup status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
