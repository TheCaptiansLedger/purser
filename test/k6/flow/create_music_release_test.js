// k6 flow: "add a music release" — the task a UI/admin tool runs when
// importing a release into the library: register the Artist, then the
// Release Group (album) under it, then the specific MusicRelease
// (pressing/edition), then a Track on that release, then tag the Release
// Group with a genre. See docs/adr/0021-music-domain-model.md for why
// Artist/Release Group/Track reuse LibraryEntry/Group/Item unmodified and
// MusicRelease is the one new tier.
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
  'purser/domain/v1/tag.proto',
  'purser/domain/v1/tag_assignment.proto',
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
        name: 'Nightglass Parade',
        overview: 'A four-piece dream-pop outfit formed in 2018.',
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
        title: 'Hollow Light',
        overview: "Nightglass Parade's sophomore studio album.",
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateGroup(release group)'
  );
  const groupId = createdGroup.group.id;

  const createdRelease = invoke(
    'purser.music.v1.MusicReleaseService/CreateMusicRelease',
    {
      musicRelease: {
        groupId: groupId,
        libraryEntryId: artistId,
        title: 'Hollow Light (2024 Remaster)',
        country: 'US',
        date: '2024-09-13T00:00:00Z',
        label: 'Twilight Media',
        catalogNumber: `TM-${suffix}`,
        format: 'CD',
        mediumCount: 1,
        trackCount: 1,
        isDefault: true,
        monitored: true,
        status: 'RELEASE_STATUS_STUB',
        mbid: `k6-flow-mbid-${suffix}`,
        barcode: `k6-flow-barcode-${suffix}`,
      },
    },
    'CreateMusicRelease'
  );
  const releaseId = createdRelease.musicRelease.id;

  const createdTrack = invoke(
    'purser.domain.v1.ItemService/CreateItem',
    {
      item: {
        contentType: 'music',
        libraryEntryId: artistId,
        groupId: groupId,
        title: 'Static Bloom',
        status: 'ITEM_STATUS_IMPORTED',
        sequence: '1',
        metadata: { release_id: releaseId },
      },
    },
    'CreateItem(track)'
  );
  const trackId = createdTrack.item.id;

  // Genre Tag attaches at the Release Group level, per
  // docs/adr/0021-music-domain-model.md's "Reuse, unmodified" table. Value
  // is suffixed per-VU: CreateTag is get-or-create on (scope, key, value)
  // (docs/adr/0019), so a literal value would make concurrent VUs share
  // one tag and race on this flow's own teardown DeleteTag.
  const createdGenreTag = invoke(
    'purser.domain.v1.TagService/CreateTag',
    { tag: { key: 'genre', value: `Dream Pop ${suffix}`, scope: 'TAG_SCOPE_METADATA', category: 'Genre' } },
    'CreateTag(genre)'
  );
  const genreTagId = createdGenreTag.tag.id;
  invoke(
    'purser.domain.v1.TagAssignmentService/CreateTagAssignment',
    { tagAssignment: { tagId: genreTagId, entityType: 'ENTITY_TYPE_GROUP', entityId: groupId } },
    'CreateTagAssignment(release group)'
  );

  // Verify the chain reads back correctly end to end.
  const fetchedRelease = invoke('purser.music.v1.MusicReleaseService/GetMusicRelease', { id: releaseId }, 'GetMusicRelease');
  check(fetchedRelease, {
    'GetMusicRelease returns the created title': (m) => m && m.musicRelease && m.musicRelease.title === 'Hollow Light (2024 Remaster)',
    'GetMusicRelease returns the parent group id': (m) => m && m.musicRelease && m.musicRelease.groupId === groupId,
  });

  const releasesByGroup = invoke(
    'purser.music.v1.MusicReleaseService/ListMusicReleases',
    { groupId: groupId, pageSize: 10 },
    'ListMusicReleases(by group)'
  );
  check(releasesByGroup, {
    'list releases under the group includes the created release': (m) => m && m.musicReleases && m.musicReleases.some((r) => r.id === releaseId),
  });

  const tracksByRelease = invoke(
    'purser.music.v1.MusicReleaseService/ListMusicReleaseTracks',
    { releaseId: releaseId, pageSize: 10 },
    'ListMusicReleaseTracks'
  );
  check(tracksByRelease, {
    'list tracks under the release includes the created track': (m) => m && m.tracks && m.tracks.some((t) => t.id === trackId),
  });

  // Tags are never embedded on Group — TagAssignment is a separate
  // polymorphic join. Prove each association round-trips in both
  // directions: "this group's tags" and "everything tagged this tag" —
  // mirroring create_studio_test.js's tag round-trip check.
  const groupTags = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { entityType: 'ENTITY_TYPE_GROUP', entityId: groupId, pageSize: 10 },
    'ListTagAssignments(by release group)'
  );
  check(groupTags, {
    "the release group's tags include the created genre tag": (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.tagId === genreTagId),
  });

  const taggedWithGenreTag = invoke(
    'purser.domain.v1.TagAssignmentService/ListTagAssignments',
    { tagId: genreTagId, pageSize: 10 },
    'ListTagAssignments(by genre tag)'
  );
  check(taggedWithGenreTag, {
    'everything tagged the genre tag includes the created release group': (m) => m && m.tagAssignments && m.tagAssignments.some((ta) => ta.entityId === groupId),
  });

  // Teardown, reverse order.
  invoke(
    'purser.domain.v1.TagAssignmentService/DeleteTagAssignment',
    { tagId: genreTagId, entityType: 'ENTITY_TYPE_GROUP', entityId: groupId },
    'DeleteTagAssignment(release group)'
  );
  invoke('purser.domain.v1.TagService/DeleteTag', { id: genreTagId }, 'DeleteTag(genre)');
  invoke('purser.domain.v1.ItemService/DeleteItem', { id: trackId }, 'DeleteItem(track)');
  invoke('purser.music.v1.MusicReleaseService/DeleteMusicRelease', { id: releaseId }, 'DeleteMusicRelease');
  invoke('purser.domain.v1.GroupService/DeleteGroup', { id: groupId }, 'DeleteGroup(release group)');
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: artistId }, 'DeleteLibraryEntry(artist)');

  client.close();
};
