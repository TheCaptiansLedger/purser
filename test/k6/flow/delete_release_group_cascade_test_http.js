// k6 flow (HTTP/JSON twin of delete_release_group_cascade_test.js):
// "delete a release group" — GetGroupDeletionImpact must report both
// referrer kinds accurately before the delete happens, then DeleteGroup
// must delete the two MusicReleases outright (Release.GroupID is
// required, so "detach" isn't a valid state for it) while only detaching
// the two Tracks (Item.GroupId and Item.Metadata.release_id cleared, not
// the Items themselves). See
// docs/adr/0015-deletion-impact-and-composing-services.md and
// docs/adr/0021-music-domain-model.md's "Ripple effects" section.
//
// Same fixture data as delete_release_group_cascade_test.js — see that
// script's header for the sourcing note.
import http from 'k6/http';
import { check, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const GROUP = `${BASE_URL}/purser.domain.v1.GroupService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
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
        name: 'Cinder & Salt',
        overview: 'A rotating-lineup post-rock collective.',
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
        title: 'Ashfall Fields',
        overview: 'A double-EP release with two distinct pressings.',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateGroup(release group)'
  );
  const groupId = createdGroup.json('group.id');

  // Two releases (pressings) under the one Release Group, one Track each.
  const releaseIds = [];
  const trackIds = [];
  for (let i = 0; i < 2; i++) {
    const createdRelease = invoke(
      `${MUSIC_RELEASE}/CreateMusicRelease`,
      {
        musicRelease: {
          groupId: groupId,
          libraryEntryId: artistId,
          title: `Ashfall Fields (Pressing ${i})`,
          format: i === 0 ? 'Vinyl' : 'CD',
          status: 'RELEASE_STATUS_STUB',
          mbid: `k6-cascade-http-mbid-${i}-${suffix}`,
          barcode: `k6-cascade-http-barcode-${i}-${suffix}`,
        },
      },
      `CreateMusicRelease[${i}]`
    );
    releaseIds.push(createdRelease.json('musicRelease.id'));

    const createdTrack = invoke(
      `${ITEM}/CreateItem`,
      {
        item: {
          contentType: 'music',
          libraryEntryId: artistId,
          groupId: groupId,
          title: `Cascade Track ${i}`,
          status: 'ITEM_STATUS_IMPORTED',
          sequence: `${i + 1}`,
          metadata: { release_id: releaseIds[i] },
        },
      },
      `CreateItem(track ${i})`
    );
    trackIds.push(createdTrack.json('item.id'));
  }

  // GetGroupDeletionImpact: two Releases, two Tracks — assert both
  // referrer kinds and counts before anything is deleted.
  const impact = invoke(`${GROUP}/GetGroupDeletionImpact`, { id: groupId }, 'GetGroupDeletionImpact');
  check(impact, {
    'GetGroupDeletionImpact reports 2 items': (r) => (r.json('impacts') || []).some((i) => i.kind === 'item' && i.count === 2),
    'GetGroupDeletionImpact reports 2 music_releases': (r) => (r.json('impacts') || []).some((i) => i.kind === 'music_release' && i.count === 2),
  });

  invoke(`${GROUP}/DeleteGroup`, { id: groupId }, 'DeleteGroup');

  let res = http.post(`${GROUP}/GetGroup`, JSON.stringify({ id: groupId }), HEADERS);
  check(res, { 'GetGroup after Delete is 404 (NotFound)': (r) => r.status === 404 });

  // Both Releases must be gone — Group deletion deletes referencing
  // Releases outright, since GroupID is required and can't be detached.
  for (const releaseId of releaseIds) {
    res = http.post(`${MUSIC_RELEASE}/GetMusicRelease`, JSON.stringify({ id: releaseId }), HEADERS);
    check(res, { [`GetMusicRelease(${releaseId}) after Group Delete is 404 (NotFound)`]: (r) => r.status === 404 });
  }

  // Both Tracks must survive, just detached — each Release's own Unlink
  // step runs as part of Group deletion, clearing metadata.release_id,
  // and Group deletion itself detaches (not deletes) Items.
  for (const trackId of trackIds) {
    res = invoke(`${ITEM}/GetItem`, { id: trackId }, `GetItem(${trackId}) after Group Delete`);
    check(res, {
      [`GetItem(${trackId}) after Group Delete shows groupId cleared`]: (r) => !r.json('item.groupId'),
      [`GetItem(${trackId}) after Group Delete shows metadata.release_id cleared`]: (r) => !('release_id' in (r.json('item.metadata') || {})),
    });
  }

  // Cleanup: the artist and the two orphaned tracks.
  for (const trackId of trackIds) {
    invoke(`${ITEM}/DeleteItem`, { id: trackId }, `cleanup: DeleteItem(${trackId})`);
  }
  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: artistId }, 'cleanup: DeleteLibraryEntry(artist)');
};
