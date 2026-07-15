// k6 gRPC suite for MusicReleaseService. See test/k6/grpc/group_test.js
// for the pattern this follows. This file is extended by every later
// sub-issue in the Music Release API epic, not replaced. See
// docs/adr/0021-music-domain-model.md.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/music/v1/release.proto',
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/group.proto'
);

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // A MusicRelease requires a real LibraryEntry (artist) and Group (release
  // group) to attach to — created inline the same way library_entry_test.js
  // sets up a Group under a LibraryEntry.
  let res = invoke('purser.domain.v1.LibraryEntryService/CreateLibraryEntry', {
    libraryEntry: { contentType: 'music', kind: 'artist', name: 'K6 gRPC Artist', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, { 'CreateLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK });
  const libraryEntryId = res.message.libraryEntry.id;

  res = invoke('purser.domain.v1.GroupService/CreateGroup', {
    group: { libraryEntryId: libraryEntryId, title: 'K6 gRPC Release Group', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, { 'CreateGroup status is OK': (r) => r && r.status === grpc.StatusOK });
  const groupId = res.message.group.id;

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response. A
  // caller-supplied id is explicitly sent here and must be discarded.
  res = invoke('purser.music.v1.MusicReleaseService/CreateMusicRelease', {
    musicRelease: {
      id: 'should-be-ignored',
      groupId: groupId,
      libraryEntryId: libraryEntryId,
      title: 'K6 gRPC Release',
      status: 'RELEASE_STATUS_STUB',
    },
  });
  check(res, {
    'CreateMusicRelease status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateMusicRelease returns an id': (r) => r && r.message && r.message.musicRelease && !!r.message.musicRelease.id,
    'CreateMusicRelease discards the caller-supplied id': (r) => r && r.message && r.message.musicRelease && r.message.musicRelease.id !== 'should-be-ignored',
  });
  const id = res.message.musicRelease.id;

  res = invoke('purser.music.v1.MusicReleaseService/GetMusicRelease', { id: id });
  check(res, {
    'GetMusicRelease status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetMusicRelease returns the created title': (r) => r && r.message && r.message.musicRelease && r.message.musicRelease.title === 'K6 gRPC Release',
  });

  res = invoke('purser.music.v1.MusicReleaseService/UpdateMusicRelease', {
    musicRelease: { id: id, title: 'K6 gRPC Release Updated' },
    updateMask: 'title',
  });
  check(res, {
    'UpdateMusicRelease status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateMusicRelease applied the field-masked title': (r) => r && r.message && r.message.musicRelease && r.message.musicRelease.title === 'K6 gRPC Release Updated',
  });

  res = invoke('purser.music.v1.MusicReleaseService/ListMusicReleases', { pageSize: 10 });
  check(res, {
    'ListMusicReleases status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListMusicReleases includes the updated release': (r) => r && r.message && r.message.musicReleases && r.message.musicReleases.some((rel) => rel.id === id),
  });

  res = invoke('purser.music.v1.MusicReleaseService/DeleteMusicRelease', { id: id });
  check(res, { 'DeleteMusicRelease status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.music.v1.MusicReleaseService/GetMusicRelease', { id: id });
  check(res, { 'GetMusicRelease after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
