// k6 flow: "delete a release group" — the task a UI/admin tool runs when
// an admin removes an entire album, and everything under it must resolve
// correctly per docs/adr/0015-deletion-impact-and-composing-services.md
// and docs/adr/0021-music-domain-model.md's "Ripple effects" section:
// GetGroupDeletionImpact must report both referrer kinds accurately
// before the delete happens, then DeleteGroup must delete the two
// MusicReleases outright (Release.GroupID is required, so "detach" isn't
// a valid state for it) while only detaching the two Tracks (Item.GroupId
// and Item.Metadata.release_id cleared, not the Items themselves).
//
// Fixture data is fictionalized but shape-accurate.
import grpc from 'k6/net/grpc';
import { check, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/group.proto',
  'purser/domain/v1/item.proto',
  'purser/music/v1/release.proto'
);

function invoke(method, request, label) {
  const res = client.invoke(method, request);
  const ok = check(res, { [`${label} status is OK`]: (r) => r && r.status === grpc.StatusOK });
  if (!ok) {
    fail(`${label} failed: ${res && res.status} ${res && res.error && res.error.message}`);
  }
  console.log(JSON.stringify({
    method: method,
    request: request,
    response: res.message,
  }, null, 2))
  return res.message;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const suffix = `${__VU}-${__ITER}-${Date.now()}`;

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  const createdArtist = invoke(
    'purser.domain.v1.LibraryEntryService/CreateLibraryEntry',
    {
      libraryEntry: {
        contentType: 'music',
        kind: 'artist',
        name: 'Cinder & Salt',
        overview: 'A rotating-lineup post-rock collective.',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateLibraryEntry(artist)'
  );
  const artistId = createdArtist.libraryEntry.id;

  const createdGroup = invoke(
    'purser.domain.v1.GroupService/CreateGroup',
    {
      group: {
        libraryEntryId: artistId,
        title: 'Ashfall Fields',
        overview: 'A double-EP release with two distinct pressings.',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateGroup(release group)'
  );
  const groupId = createdGroup.group.id;

  // Two releases (pressings) under the one Release Group, one Track each.
  const releases = [];
  const tracks = [];
  for (let i = 0; i < 2; i++) {
    const createdRelease = invoke(
      'purser.music.v1.MusicReleaseService/CreateMusicRelease',
      {
        musicRelease: {
          groupId: groupId,
          libraryEntryId: artistId,
          title: `Ashfall Fields (Pressing ${i})`,
          format: i === 0 ? 'Vinyl' : 'CD',
          status: 'RELEASE_STATUS_STUB',
          mbid: `k6-cascade-mbid-${i}-${suffix}`,
          barcode: `k6-cascade-barcode-${i}-${suffix}`,
        },
      },
      `CreateMusicRelease[${i}]`
    );
    const release = createdRelease.musicRelease;
    releases.push(release);

    const createdTrack = invoke(
      'purser.domain.v1.ItemService/CreateItem',
      {
        item: {
          contentType: 'music',
          libraryEntryId: artistId,
          groupId: groupId,
          title: `Cascade Track ${i}`,
          status: 'ITEM_STATUS_IMPORTED',
          sequence: `${i + 1}`,
          metadata: { release_id: release.id },
        },
      },
      `CreateItem(track ${i})`
    );
    tracks.push(createdTrack.item);
  }

  // GetGroupDeletionImpact: two Releases, two Tracks — assert both
  // referrer kinds and counts before anything is deleted.
  const impact = invoke('purser.domain.v1.GroupService/GetGroupDeletionImpact', { id: groupId }, 'GetGroupDeletionImpact');
  check(impact, {
    'GetGroupDeletionImpact reports 2 items': (m) => m && m.impacts && m.impacts.some((i) => i.kind === 'item' && i.count === 2),
    'GetGroupDeletionImpact reports 2 music_releases': (m) => m && m.impacts && m.impacts.some((i) => i.kind === 'music_release' && i.count === 2),
  });

  invoke('purser.domain.v1.GroupService/DeleteGroup', { id: groupId }, 'DeleteGroup');

  const getAfterDelete = client.invoke('purser.domain.v1.GroupService/GetGroup', { id: groupId });
  check(getAfterDelete, { 'GetGroup after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // Both Releases must be gone — Group deletion deletes referencing
  // Releases outright, since GroupID is required and can't be detached.
  for (const release of releases) {
    const r = client.invoke('purser.music.v1.MusicReleaseService/GetMusicRelease', { id: release.id });
    check(r, { [`GetMusicRelease(${release.id}) after Group Delete is NotFound`]: (res) => res && res.status === grpc.StatusNotFound });
  }

  // Both Tracks must survive, just detached — each Release's own Unlink
  // step runs as part of Group deletion, clearing metadata.release_id,
  // and Group deletion itself detaches (not deletes) Items.
  for (const track of tracks) {
    const r = client.invoke('purser.domain.v1.ItemService/GetItem', { id: track.id });
    check(r, {
      [`GetItem(${track.id}) after Group Delete still finds the track (detached, not deleted)`]: (res) => res && res.status === grpc.StatusOK,
      [`GetItem(${track.id}) after Group Delete shows groupId cleared`]: (res) => res && res.message && res.message.item && res.message.item.groupId === '',
      [`GetItem(${track.id}) after Group Delete shows metadata.release_id cleared`]: (res) =>
        res && res.message && res.message.item && !(res.message.item.metadata && 'release_id' in res.message.item.metadata),
    });
  }

  // Cleanup: the artist and the two orphaned tracks.
  for (const track of tracks) {
    invoke('purser.domain.v1.ItemService/DeleteItem', { id: track.id }, `cleanup: DeleteItem(${track.id})`);
  }
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: artistId }, 'cleanup: DeleteLibraryEntry(artist)');

  client.close();
};
