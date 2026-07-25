// k6 flow (HTTP/JSON twin of create_music_release_test.js): "add a music
// release" — register the Artist, then the Release Group under it, then
// the specific MusicRelease (pressing/edition), then a Track on that
// release, then tag the Release Group with a genre. See
// docs/adr/0021-music-domain-model.md for why Artist/Release
// Group/Track reuse LibraryEntry/Group/Item unmodified and MusicRelease
// is the one new tier.
//
// Same fixture data as create_music_release_test.js — see that script's
// header for the sourcing note.
import http from 'k6/http';
import { check, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const GROUP = `${BASE_URL}/purser.domain.v1.GroupService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
const TAG = `${BASE_URL}/purser.domain.v1.TagService`;
const TAG_ASSIGNMENT = `${BASE_URL}/purser.domain.v1.TagAssignmentService`;
const MUSIC_RELEASE = `${BASE_URL}/purser.music.v1.MusicReleaseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, label) {
  const res = http.post(url, JSON.stringify(body), HEADERS);
  const ok = check(res, { [`${label} status is 200`]: (r) => r.status === 200 });
  if (!ok) {
    fail(`${label} failed: ${res.status} ${res.body}`);
  }
  console.log(JSON.stringify({
    method: url,
    request: body,
    response: res.json(),
  }, null, 2))
  return res;
}

export default () => {
  const suffix = `${__VU}-${__ITER}-${Date.now()}`;

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  const createdArtist = invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
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
  const artistId = createdArtist.json('libraryEntry.id');

  const createdGroup = invoke(
    `${GROUP}/CreateGroup`,
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
  const groupId = createdGroup.json('group.id');

  const createdRelease = invoke(
    `${MUSIC_RELEASE}/CreateMusicRelease`,
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
        mbid: `k6-flow-http-mbid-${suffix}`,
        barcode: `k6-flow-http-barcode-${suffix}`,
      },
    },
    'CreateMusicRelease'
  );
  const releaseId = createdRelease.json('musicRelease.id');

  const createdTrack = invoke(
    `${ITEM}/CreateItem`,
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
  const trackId = createdTrack.json('item.id');

  // Genre Tag attaches at the Release Group level, per
  // docs/adr/0021-music-domain-model.md's "Reuse, unmodified" table. Value
  // is suffixed per-VU: CreateTag is get-or-create on (scope, key, value)
  // (docs/adr/0019), so a literal value would make concurrent VUs share
  // one tag and race on this flow's own teardown DeleteTag.
  const createdGenreTag = invoke(
    `${TAG}/CreateTag`,
    { tag: { key: 'genre', value: `Dream Pop ${suffix}`, scope: 'TAG_SCOPE_METADATA', category: 'Genre' } },
    'CreateTag(genre)'
  );
  const genreTagId = createdGenreTag.json('tag.id');
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: genreTagId, entityType: 'ENTITY_TYPE_GROUP', entityId: groupId } },
    'CreateTagAssignment(release group)'
  );

  // Verify the chain reads back correctly end to end.
  let res = invoke(`${MUSIC_RELEASE}/GetMusicRelease`, { id: releaseId }, 'GetMusicRelease');
  check(res, {
    'GetMusicRelease returns the created title': (r) => r.json('musicRelease.title') === 'Hollow Light (2024 Remaster)',
    'GetMusicRelease returns the parent group id': (r) => r.json('musicRelease.groupId') === groupId,
  });

  res = invoke(`${MUSIC_RELEASE}/ListMusicReleases`, { groupId: groupId, pageSize: 10 }, 'ListMusicReleases(by group)');
  check(res, {
    'list releases under the group includes the created release': (r) => (r.json('musicReleases') || []).some((rel) => rel.id === releaseId),
  });

  res = invoke(`${MUSIC_RELEASE}/ListMusicReleaseTracks`, { releaseId: releaseId, pageSize: 10 }, 'ListMusicReleaseTracks');
  check(res, {
    'list tracks under the release includes the created track': (r) => (r.json('tracks') || []).some((t) => t.id === trackId),
  });

  // Tags are never embedded on Group — TagAssignment is a separate
  // polymorphic join. Prove each association round-trips in both
  // directions: "this group's tags" and "everything tagged this tag" —
  // mirroring create_studio_http_test.js's tag round-trip check.
  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { entityType: 'ENTITY_TYPE_GROUP', entityId: groupId, pageSize: 10 }, 'ListTagAssignments(by release group)');
  check(res, {
    "the release group's tags include the created genre tag": (r) => (r.json('tagAssignments') || []).some((ta) => ta.tagId === genreTagId),
  });

  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { tagId: genreTagId, pageSize: 10 }, 'ListTagAssignments(by genre tag)');
  check(res, {
    'everything tagged the genre tag includes the created release group': (r) => (r.json('tagAssignments') || []).some((ta) => ta.entityId === groupId),
  });

  // Teardown, reverse order.
  invoke(
    `${TAG_ASSIGNMENT}/DeleteTagAssignment`,
    { tagId: genreTagId, entityType: 'ENTITY_TYPE_GROUP', entityId: groupId },
    'DeleteTagAssignment(release group)'
  );
  invoke(`${TAG}/DeleteTag`, { id: genreTagId }, 'DeleteTag(genre)');
  invoke(`${ITEM}/DeleteItem`, { id: trackId }, 'DeleteItem(track)');
  invoke(`${MUSIC_RELEASE}/DeleteMusicRelease`, { id: releaseId }, 'DeleteMusicRelease');
  invoke(`${GROUP}/DeleteGroup`, { id: groupId }, 'DeleteGroup(release group)');
  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: artistId }, 'DeleteLibraryEntry(artist)');
};
