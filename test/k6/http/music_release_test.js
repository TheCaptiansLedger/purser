// k6 HTTP/JSON suite for MusicReleaseService — Connect's HTTP/JSON
// transport. See test/k6/http/group_test.js for the pattern this follows.
// This file is extended by every later sub-issue in the Music Release API
// epic, not replaced. See docs/adr/0021-music-domain-model.md.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY_SERVICE = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const GROUP_SERVICE = `${BASE_URL}/purser.domain.v1.GroupService`;
const ITEM_SERVICE = `${BASE_URL}/purser.domain.v1.ItemService`;
const MUSIC_RELEASE_SERVICE = `${BASE_URL}/purser.music.v1.MusicReleaseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  // A MusicRelease requires a real LibraryEntry (artist) and Group (release
  // group) to attach to — created inline the same way library_entry_test.js
  // sets up a Group under a LibraryEntry.
  let res = invoke(
    `${LIBRARY_ENTRY_SERVICE}/CreateLibraryEntry`,
    JSON.stringify({ libraryEntry: { contentType: 'music', kind: 'artist', name: 'K6 HTTP Artist', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateLibraryEntry status is 200': (r) => r.status === 200 });
  const libraryEntryId = res.json('libraryEntry.id');

  res = invoke(
    `${GROUP_SERVICE}/CreateGroup`,
    JSON.stringify({ group: { libraryEntryId: libraryEntryId, title: 'K6 HTTP Release Group', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateGroup status is 200': (r) => r.status === 200 });
  const groupId = res.json('group.id');

  // Seed 3 releases with distinct MBIDs/barcodes under the one Group, to
  // prove GetByMBID/GetByBarcode/ListByGroup/ListByEntry each return the
  // correct release(s), not just "a" release.
  const seeded = [];
  for (let i = 0; i < 3; i++) {
    // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
    // — never sent on Create, always read back from the response. A
    // caller-supplied id is explicitly sent here (on the first iteration)
    // and must be discarded.
    res = invoke(
      `${MUSIC_RELEASE_SERVICE}/CreateMusicRelease`,
      JSON.stringify({
        musicRelease: {
          id: i === 0 ? 'should-be-ignored' : '',
          groupId: groupId,
          libraryEntryId: libraryEntryId,
          title: `K6 HTTP Release ${i}`,
          status: 'RELEASE_STATUS_STUB',
          mbid: `k6-http-mbid-${i}-${__VU}-${__ITER}`,
          barcode: `k6-http-barcode-${i}-${__VU}-${__ITER}`,
        },
      }),
      HEADERS
    );
    check(res, {
      [`CreateMusicRelease[${i}] status is 200`]: (r) => r.status === 200,
      [`CreateMusicRelease[${i}] returns an id`]: (r) => !!r.json('musicRelease.id'),
    });
    if (i === 0) {
      check(res, { 'CreateMusicRelease discards the caller-supplied id': (r) => r.json('musicRelease.id') !== 'should-be-ignored' });
    }
    seeded.push(res.json('musicRelease'));
  }
  const id = seeded[0].id;

  res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicRelease`, JSON.stringify({ id: id }), HEADERS);
  check(res, {
    'GetMusicRelease status is 200': (r) => r.status === 200,
    'GetMusicRelease returns the created title': (r) => r.json('musicRelease.title') === 'K6 HTTP Release 0',
  });

  // GetMusicReleaseByMBID/GetMusicReleaseByBarcode: for each of the 3
  // seeded releases, prove the lookup returns the correct release, not
  // just any release under the shared Group.
  for (const rel of seeded) {
    res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicReleaseByMBID`, JSON.stringify({ mbid: rel.mbid }), HEADERS);
    check(res, {
      [`GetMusicReleaseByMBID(${rel.mbid}) status is 200`]: (r) => r.status === 200,
      [`GetMusicReleaseByMBID(${rel.mbid}) returns the matching release`]: (r) => r.json('musicRelease.id') === rel.id,
    });

    res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicReleaseByBarcode`, JSON.stringify({ barcode: rel.barcode }), HEADERS);
    check(res, {
      [`GetMusicReleaseByBarcode(${rel.barcode}) status is 200`]: (r) => r.status === 200,
      [`GetMusicReleaseByBarcode(${rel.barcode}) returns the matching release`]: (r) => r.json('musicRelease.id') === rel.id,
    });
  }

  res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicReleaseByMBID`, JSON.stringify({ mbid: 'no-such-mbid' }), HEADERS);
  check(res, { 'GetMusicReleaseByMBID on unknown MBID is 404 (NotFound)': (r) => r.status === 404 });

  res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicReleaseByBarcode`, JSON.stringify({ barcode: 'no-such-barcode' }), HEADERS);
  check(res, { 'GetMusicReleaseByBarcode on unknown barcode is 404 (NotFound)': (r) => r.status === 404 });

  // ListMusicReleases filtered by group_id: exactly the 3 seeded releases,
  // no others.
  res = invoke(`${MUSIC_RELEASE_SERVICE}/ListMusicReleases`, JSON.stringify({ pageSize: 10, groupId: groupId }), HEADERS);
  check(res, {
    'ListMusicReleases(group_id) status is 200': (r) => r.status === 200,
    'ListMusicReleases(group_id) returns exactly the 3 seeded releases': (r) => {
      const ids = (r.json('musicReleases') || []).map((rel) => rel.id).sort();
      return ids.length === 3 && ids.join(',') === seeded.map((rel) => rel.id).sort().join(',');
    },
  });

  // ListMusicReleases filtered by library_entry_id: same 3 releases, since
  // they all share the one seeded LibraryEntry.
  res = invoke(`${MUSIC_RELEASE_SERVICE}/ListMusicReleases`, JSON.stringify({ pageSize: 10, libraryEntryId: libraryEntryId }), HEADERS);
  check(res, {
    'ListMusicReleases(library_entry_id) status is 200': (r) => r.status === 200,
    'ListMusicReleases(library_entry_id) returns exactly the 3 seeded releases': (r) => {
      const ids = (r.json('musicReleases') || []).map((rel) => rel.id).sort();
      return ids.length === 3 && ids.join(',') === seeded.map((rel) => rel.id).sort().join(',');
    },
  });

  // ListMusicReleaseTracks: prove Track ↔ Release linkage and isolation.
  // Tracks are created via the existing, unmodified
  // purser.domain.v1.ItemService/CreateItem — contentType "music", GroupId
  // set to the shared Release Group, Metadata.release_id set to the
  // specific release a track belongs to. See
  // docs/adr/0021-music-domain-model.md's "Track ↔ Release linkage"
  // section.
  const releaseA = seeded[0];
  const releaseB = seeded[1];

  res = invoke(
    `${ITEM_SERVICE}/CreateItem`,
    JSON.stringify({
      item: {
        contentType: 'music',
        libraryEntryId: libraryEntryId,
        groupId: groupId,
        title: 'K6 HTTP Track A',
        status: 'ITEM_STATUS_IMPORTED',
        metadata: { release_id: releaseA.id },
      },
    }),
    HEADERS
  );
  check(res, {
    'CreateItem(trackA) status is 200': (r) => r.status === 200,
    'CreateItem(trackA) returns an id': (r) => !!r.json('item.id'),
  });
  const trackA = res.json('item');

  res = invoke(`${MUSIC_RELEASE_SERVICE}/ListMusicReleaseTracks`, JSON.stringify({ releaseId: releaseA.id, pageSize: 10 }), HEADERS);
  check(res, {
    'ListMusicReleaseTracks(releaseA) status is 200': (r) => r.status === 200,
    'ListMusicReleaseTracks(releaseA) includes trackA': (r) => (r.json('tracks') || []).some((i) => i.id === trackA.id),
  });

  // A second Track under releaseB — same Release Group as trackA, a
  // different release — proves isolation is by release_id, not just by
  // group_id.
  res = invoke(
    `${ITEM_SERVICE}/CreateItem`,
    JSON.stringify({
      item: {
        contentType: 'music',
        libraryEntryId: libraryEntryId,
        groupId: groupId,
        title: 'K6 HTTP Track B',
        status: 'ITEM_STATUS_IMPORTED',
        metadata: { release_id: releaseB.id },
      },
    }),
    HEADERS
  );
  check(res, {
    'CreateItem(trackB) status is 200': (r) => r.status === 200,
    'CreateItem(trackB) returns an id': (r) => !!r.json('item.id'),
  });
  const trackB = res.json('item');

  res = invoke(`${MUSIC_RELEASE_SERVICE}/ListMusicReleaseTracks`, JSON.stringify({ releaseId: releaseA.id, pageSize: 10 }), HEADERS);
  check(res, {
    'ListMusicReleaseTracks(releaseA) after trackB status is 200': (r) => r.status === 200,
    'ListMusicReleaseTracks(releaseA) still includes trackA': (r) => (r.json('tracks') || []).some((i) => i.id === trackA.id),
    'ListMusicReleaseTracks(releaseA) excludes trackB': (r) => !(r.json('tracks') || []).some((i) => i.id === trackB.id),
  });

  res = invoke(
    `${MUSIC_RELEASE_SERVICE}/UpdateMusicRelease`,
    JSON.stringify({ musicRelease: { id: id, title: 'K6 HTTP Release Updated' }, updateMask: 'title' }),
    HEADERS
  );
  check(res, {
    'UpdateMusicRelease status is 200': (r) => r.status === 200,
    'UpdateMusicRelease applied the field-masked title': (r) => r.json('musicRelease.title') === 'K6 HTTP Release Updated',
  });

  res = invoke(`${MUSIC_RELEASE_SERVICE}/ListMusicReleases`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListMusicReleases status is 200': (r) => r.status === 200,
    'ListMusicReleases includes the updated release': (r) => (r.json('musicReleases') || []).some((rel) => rel.id === id),
  });

  // GetMusicReleaseDeletionImpact/DeleteMusicRelease: releaseA (== id) owns
  // trackA, so its impact must report exactly one referencing item ("Tracks").
  // See docs/adr/0015-deletion-impact-and-composing-services.md.
  res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicReleaseDeletionImpact`, JSON.stringify({ id: releaseA.id }), HEADERS);
  check(res, {
    'GetMusicReleaseDeletionImpact status is 200': (r) => r.status === 200,
    'GetMusicReleaseDeletionImpact reports the track': (r) => (r.json('impacts') || []).some((i) => i.kind === 'item' && i.count === 1),
  });

  for (const rel of seeded) {
    res = invoke(`${MUSIC_RELEASE_SERVICE}/DeleteMusicRelease`, JSON.stringify({ id: rel.id }), HEADERS);
    check(res, { [`DeleteMusicRelease(${rel.id}) status is 200`]: (r) => r.status === 200 });
  }

  res = invoke(`${MUSIC_RELEASE_SERVICE}/GetMusicRelease`, JSON.stringify({ id: id }), HEADERS);
  check(res, { 'GetMusicRelease after Delete is 404 (NotFound)': (r) => r.status === 404 });

  // trackA must survive releaseA's deletion, just detached — Unlink clears
  // metadata.release_id rather than deleting the track.
  res = invoke(`${ITEM_SERVICE}/GetItem`, JSON.stringify({ id: trackA.id }), HEADERS);
  check(res, {
    'GetItem after MusicRelease Delete still finds trackA (detached, not deleted)': (r) => r.status === 200,
    'GetItem after MusicRelease Delete shows metadata.release_id cleared': (r) => {
      const metadata = r.json('item.metadata');
      return !metadata || !('release_id' in metadata);
    },
  });

  res = invoke(`${ITEM_SERVICE}/DeleteItem`, JSON.stringify({ id: trackA.id }), HEADERS);
  check(res, { 'cleanup: DeleteItem(trackA) status is 200': (r) => r.status === 200 });
  res = invoke(`${ITEM_SERVICE}/DeleteItem`, JSON.stringify({ id: trackB.id }), HEADERS);
  check(res, { 'cleanup: DeleteItem(trackB) status is 200': (r) => r.status === 200 });
};
