// k6 HTTP/JSON suite for GroupService. See test/k6/http/person_test.js
// for the pattern this follows.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.domain.v1.GroupService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
const MUSIC_RELEASE = `${BASE_URL}/purser.music.v1.MusicReleaseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  let res = invoke(
    `${SERVICE}/CreateGroup`,
    JSON.stringify({ group: { libraryEntryId: 'entry1', title: 'K6 HTTP Group', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, {
    'CreateGroup status is 200': (r) => r.status === 200,
    'CreateGroup returns an id': (r) => !!r.json('group.id'),
  });
  const id = res.json('group.id');

  res = invoke(`${SERVICE}/GetGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetGroup status is 200': (r) => r.status === 200,
    'GetGroup returns the created title': (r) => r.json('group.title') === 'K6 HTTP Group',
  });

  res = invoke(`${SERVICE}/UpdateGroup`, JSON.stringify({ group: { id: id, title: 'K6 HTTP Group Updated' }, updateMask: 'title' }), HEADERS);
  check(res, {
    'UpdateGroup status is 200': (r) => r.status === 200,
    'UpdateGroup applied the field-masked title': (r) => r.json('group.title') === 'K6 HTTP Group Updated',
  });

  res = invoke(`${SERVICE}/ListGroups`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListGroups status is 200': (r) => r.status === 200,
    'ListGroups includes the created group': (r) => (r.json('groups') || []).some((g) => g.id === id),
  });

  res = invoke(
    `${ITEM}/CreateItem`,
    JSON.stringify({ item: { contentType: 'adult', libraryEntryId: 'entry1', groupId: id, title: 'K6 Group Deletion Item', status: 'ITEM_STATUS_WANTED' } }),
    HEADERS
  );
  check(res, {
    'CreateItem status is 200': (r) => r.status === 200,
    'CreateItem returns an id': (r) => !!r.json('item.id'),
  });
  const itemId = res.json('item.id');

  // A MusicRelease under this Group, plus a track pointing at it — proves
  // GroupDeletionService's other referrer: since music.Release.GroupID is
  // required (unlike Item.GroupID), Group deletion must delete the
  // Release outright (not detach it), while unlinking the Release's own
  // tracks rather than deleting them. See
  // docs/adr/0021-music-domain-model.md's "Ripple effects" section.
  res = invoke(
    `${MUSIC_RELEASE}/CreateMusicRelease`,
    JSON.stringify({ musicRelease: { groupId: id, libraryEntryId: 'entry1', title: 'K6 Group Deletion Release', status: 'RELEASE_STATUS_STUB' } }),
    HEADERS
  );
  check(res, {
    'CreateMusicRelease status is 200': (r) => r.status === 200,
    'CreateMusicRelease returns an id': (r) => !!r.json('musicRelease.id'),
  });
  const releaseId = res.json('musicRelease.id');

  res = invoke(
    `${ITEM}/CreateItem`,
    JSON.stringify({
      item: { contentType: 'music', libraryEntryId: 'entry1', groupId: id, title: 'K6 Group Deletion Track', status: 'ITEM_STATUS_IMPORTED', metadata: { release_id: releaseId } },
    }),
    HEADERS
  );
  check(res, {
    'CreateItem(track) status is 200': (r) => r.status === 200,
    'CreateItem(track) returns an id': (r) => !!r.json('item.id'),
  });
  const trackId = res.json('item.id');

  res = invoke(`${SERVICE}/GetGroupDeletionImpact`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetGroupDeletionImpact status is 200': (r) => r.status === 200,
    'GetGroupDeletionImpact reports the item': (r) => (r.json('impacts') || []).some((i) => i.kind === 'item' && i.count === 2),
    'GetGroupDeletionImpact reports the music_release': (r) => (r.json('impacts') || []).some((i) => i.kind === 'music_release' && i.count === 1),
  });

  res = invoke(`${SERVICE}/DeleteGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'DeleteGroup status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetGroup`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetGroup after Delete is 404 (NotFound)': (r) => r.status === 404 });

  // The item must still exist, just detached (groupId cleared) — Group
  // deletion detaches Items rather than deleting them.
  res = invoke(`${ITEM}/GetItem`, JSON.stringify({ id: itemId }), HEADERS);
  check(res, {
    'GetItem after Group Delete still finds the item (detached, not deleted)': (r) => r.status === 200,
    // Connect's protojson mapping omits proto3 zero-value fields (an
    // empty string) from the response entirely, so a cleared groupId
    // shows up as undefined here, not "".
    'GetItem after Group Delete shows groupId cleared': (r) => !r.json('item.groupId'),
  });

  // The Release must be gone — Group deletion deletes referencing
  // Releases outright, since GroupID is required and can't be detached.
  res = invoke(`${MUSIC_RELEASE}/GetMusicRelease`, JSON.stringify({ id: releaseId }), HEADERS);
  check(res, { 'GetMusicRelease after Group Delete is 404 (NotFound)': (r) => r.status === 404 });

  // The track must still exist, just detached (metadata.release_id
  // cleared) — the Release's own Unlink step runs as part of Group
  // deletion.
  res = invoke(`${ITEM}/GetItem`, JSON.stringify({ id: trackId }), HEADERS);
  check(res, {
    'GetItem(track) after Group Delete still finds the track (detached, not deleted)': (r) => r.status === 200,
    'GetItem(track) after Group Delete shows metadata.release_id cleared': (r) => !(r.json('item.metadata') && 'release_id' in r.json('item.metadata')),
  });

  res = invoke(`${ITEM}/DeleteItem`, JSON.stringify({ id: itemId }), HEADERS);
  check(res, { 'cleanup: DeleteItem status is 200': (r) => r.status === 200 });

  res = invoke(`${ITEM}/DeleteItem`, JSON.stringify({ id: trackId }), HEADERS);
  check(res, { 'cleanup: DeleteItem(track) status is 200': (r) => r.status === 200 });

  // BulkDeleteGroups: the "delete these 12 duplicate discography groups"
  // use case — see docs/adr/0016-bulk-operations.md.
  const bulkIds = [];
  for (let i = 0; i < 3; i++) {
    res = invoke(
      `${SERVICE}/CreateGroup`,
      JSON.stringify({ group: { libraryEntryId: 'entry1', title: 'K6 Bulk Group', monitorMode: 'MONITOR_MODE_NONE' } }),
      HEADERS
    );
    check(res, {
      'setup: CreateGroup status is 200': (r) => r.status === 200,
      'setup: CreateGroup returns an id': (r) => !!r.json('group.id'),
    });
    bulkIds.push(res.json('group.id'));
  }
  const [bulkId1, bulkId2, bulkId3] = bulkIds;

  res = invoke(`${SERVICE}/BulkDeleteGroups`, JSON.stringify({ ids: [bulkId1, bulkId2] }), HEADERS);
  check(res, { 'BulkDeleteGroups status is 200': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetGroup`, JSON.stringify({ id: bulkId1 }), HEADERS);
  check(res, { 'GetGroup for bulkId1 after BulkDeleteGroups is 404': (r) => r.status === 404 });
  res = invoke(`${SERVICE}/GetGroup`, JSON.stringify({ id: bulkId2 }), HEADERS);
  check(res, { 'GetGroup for bulkId2 after BulkDeleteGroups is 404': (r) => r.status === 404 });
  res = invoke(`${SERVICE}/GetGroup`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'GetGroup for bulkId3 (not in the batch) still exists': (r) => r.status === 200 });

  // All-or-nothing: a batch with one missing id must fail entirely — the
  // still-existing bulkId3 must not be removed either.
  res = invoke(`${SERVICE}/BulkDeleteGroups`, JSON.stringify({ ids: [bulkId3, 'k6-http-group-missing'] }), HEADERS);
  check(res, { 'BulkDeleteGroups with a missing id is 404': (r) => r.status === 404 });

  res = invoke(`${SERVICE}/GetGroup`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'GetGroup for bulkId3 after failed batch still exists (rolled back)': (r) => r.status === 200 });

  res = invoke(`${SERVICE}/DeleteGroup`, JSON.stringify({ id: bulkId3 }), HEADERS);
  check(res, { 'cleanup: DeleteGroup status is 200': (r) => r.status === 200 });
};
