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

  // Seed 3 releases with distinct MBIDs/barcodes under the one Group, to
  // prove GetByMBID/GetByBarcode/ListByGroup/ListByEntry each return the
  // correct release(s), not just "a" release.
  const seeded = [];
  for (let i = 0; i < 3; i++) {
    // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
    // — never sent on Create, always read back from the response. A
    // caller-supplied id is explicitly sent here (on the first iteration)
    // and must be discarded.
    res = invoke('purser.music.v1.MusicReleaseService/CreateMusicRelease', {
      musicRelease: {
        id: i === 0 ? 'should-be-ignored' : '',
        groupId: groupId,
        libraryEntryId: libraryEntryId,
        title: `K6 gRPC Release ${i}`,
        status: 'RELEASE_STATUS_STUB',
        mbid: `k6-grpc-mbid-${i}-${__VU}-${__ITER}`,
        barcode: `k6-grpc-barcode-${i}-${__VU}-${__ITER}`,
      },
    });
    check(res, {
      [`CreateMusicRelease[${i}] status is OK`]: (r) => r && r.status === grpc.StatusOK,
      [`CreateMusicRelease[${i}] returns an id`]: (r) => r && r.message && r.message.musicRelease && !!r.message.musicRelease.id,
    });
    if (i === 0) {
      check(res, {
        'CreateMusicRelease discards the caller-supplied id': (r) => r && r.message && r.message.musicRelease && r.message.musicRelease.id !== 'should-be-ignored',
      });
    }
    seeded.push(res.message.musicRelease);
  }
  const id = seeded[0].id;

  res = invoke('purser.music.v1.MusicReleaseService/GetMusicRelease', { id: id });
  check(res, {
    'GetMusicRelease status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetMusicRelease returns the created title': (r) => r && r.message && r.message.musicRelease && r.message.musicRelease.title === 'K6 gRPC Release 0',
  });

  // GetMusicReleaseByMBID/GetMusicReleaseByBarcode: for each of the 3
  // seeded releases, prove the lookup returns the correct release, not
  // just any release under the shared Group.
  for (const rel of seeded) {
    res = invoke('purser.music.v1.MusicReleaseService/GetMusicReleaseByMBID', { mbid: rel.mbid });
    check(res, {
      [`GetMusicReleaseByMBID(${rel.mbid}) status is OK`]: (r) => r && r.status === grpc.StatusOK,
      [`GetMusicReleaseByMBID(${rel.mbid}) returns the matching release`]: (r) => r && r.message && r.message.musicRelease && r.message.musicRelease.id === rel.id,
    });

    res = invoke('purser.music.v1.MusicReleaseService/GetMusicReleaseByBarcode', { barcode: rel.barcode });
    check(res, {
      [`GetMusicReleaseByBarcode(${rel.barcode}) status is OK`]: (r) => r && r.status === grpc.StatusOK,
      [`GetMusicReleaseByBarcode(${rel.barcode}) returns the matching release`]: (r) => r && r.message && r.message.musicRelease && r.message.musicRelease.id === rel.id,
    });
  }

  res = invoke('purser.music.v1.MusicReleaseService/GetMusicReleaseByMBID', { mbid: 'no-such-mbid' });
  check(res, { 'GetMusicReleaseByMBID on unknown MBID is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = invoke('purser.music.v1.MusicReleaseService/GetMusicReleaseByBarcode', { barcode: 'no-such-barcode' });
  check(res, { 'GetMusicReleaseByBarcode on unknown barcode is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // ListMusicReleases filtered by group_id: exactly the 3 seeded releases,
  // no others.
  res = invoke('purser.music.v1.MusicReleaseService/ListMusicReleases', { pageSize: 10, groupId: groupId });
  check(res, {
    'ListMusicReleases(group_id) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListMusicReleases(group_id) returns exactly the 3 seeded releases': (r) => {
      const ids = (r.message.musicReleases || []).map((rel) => rel.id).sort();
      return ids.length === 3 && ids.join(',') === seeded.map((rel) => rel.id).sort().join(',');
    },
  });

  // ListMusicReleases filtered by library_entry_id: same 3 releases, since
  // they all share the one seeded LibraryEntry.
  res = invoke('purser.music.v1.MusicReleaseService/ListMusicReleases', { pageSize: 10, libraryEntryId: libraryEntryId });
  check(res, {
    'ListMusicReleases(library_entry_id) status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListMusicReleases(library_entry_id) returns exactly the 3 seeded releases': (r) => {
      const ids = (r.message.musicReleases || []).map((rel) => rel.id).sort();
      return ids.length === 3 && ids.join(',') === seeded.map((rel) => rel.id).sort().join(',');
    },
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

  for (const rel of seeded) {
    res = invoke('purser.music.v1.MusicReleaseService/DeleteMusicRelease', { id: rel.id });
    check(res, { [`DeleteMusicRelease(${rel.id}) status is OK`]: (r) => r && r.status === grpc.StatusOK });
  }

  res = invoke('purser.music.v1.MusicReleaseService/GetMusicRelease', { id: id });
  check(res, { 'GetMusicRelease after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
