// k6 flow (HTTP/JSON twin of various_artists_compilation_test.js):
// "import a Various Artists compilation" — a compilation's Release Group
// owned by MusicBrainz's well-known "Various Artists" sentinel artist,
// per docs/adr/0021-music-domain-model.md's "Various Artists
// compilations" section. Proves "everything featuring Person X"
// (ItemPerson list filtered by PersonID) returns the correct track for
// each of two differently-credited artists on the same compilation.
//
// The sentinel LibraryEntry is get-or-created inline here, mirroring
// cmd/purser/seed_various_artists.go's idempotent find-or-create logic —
// `make k6-ci` runs against a fresh, unseeded hermetic store (see
// docs/adr/0022-k6-ci-enforcement.md), so this flow can't assume a
// separate seeding step already ran. The sentinel itself is left in
// place on teardown since it's shared, idempotent infrastructure, not a
// fixture owned by this flow.
//
// Same fixture data as various_artists_compilation_test.js — see that
// script's header for the sourcing note.
import http from 'k6/http';
import { check, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const EXTERNAL_ID = `${BASE_URL}/purser.domain.v1.ExternalIDService`;
const GROUP = `${BASE_URL}/purser.domain.v1.GroupService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
const PERSON = `${BASE_URL}/purser.domain.v1.PersonService`;
const ITEM_PERSON = `${BASE_URL}/purser.domain.v1.ItemPersonService`;
const MUSIC_RELEASE = `${BASE_URL}/purser.music.v1.MusicReleaseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

const VARIOUS_ARTISTS_MBID = '89ad4ac3-39f7-470e-963a-56509c546377';
const VARIOUS_ARTISTS_NAME = 'Various Artists';

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

// ensureVariousArtistsExternalID is the same idempotency guard
// cmd/purser/seed_various_artists.go uses: Get first (a concurrent VU may
// have found the LibraryEntry via List a moment after its creator ran
// this same Create but before this VU's own attempt), and only Create on
// NotFound.
function ensureVariousArtistsExternalID(entryId) {
  const res = http.post(
    `${EXTERNAL_ID}/GetExternalID`,
    JSON.stringify({ entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: entryId, source: 'mbz' }),
    HEADERS
  );
  if (res.status === 200) {
    return;
  }
  invoke(
    `${EXTERNAL_ID}/CreateExternalID`,
    { externalId: { entityType: 'ENTITY_TYPE_LIBRARY_ENTRY', entityId: entryId, source: 'mbz', value: VARIOUS_ARTISTS_MBID } },
    'CreateExternalID(various artists sentinel)'
  );
}

// findOrCreateVariousArtists mirrors
// cmd/purser/seed_various_artists.go's idempotent find-or-create: page
// through artist LibraryEntries looking for one named "Various Artists"
// and reuse it if found (attaching/verifying its mbz ExternalID either
// way), otherwise create both.
function findOrCreateVariousArtists() {
  let pageToken = '';
  for (;;) {
    const listed = invoke(
      `${LIBRARY_ENTRY}/ListLibraryEntries`,
      { kind: 'artist', pageToken: pageToken, pageSize: 50 },
      'ListLibraryEntries(artist, sentinel lookup)'
    );
    const found = (listed.json('libraryEntries') || []).find((e) => e.name === VARIOUS_ARTISTS_NAME);
    if (found) {
      ensureVariousArtistsExternalID(found.id);
      return found.id;
    }
    pageToken = listed.json('nextPageToken');
    if (!pageToken) {
      break;
    }
  }

  const created = invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
    {
      libraryEntry: {
        contentType: 'music',
        kind: 'artist',
        name: VARIOUS_ARTISTS_NAME,
        monitorMode: 'MONITOR_MODE_NONE',
      },
    },
    'CreateLibraryEntry(various artists sentinel)'
  );
  const entryId = created.json('libraryEntry.id');

  ensureVariousArtistsExternalID(entryId);

  return entryId;
}

export default () => {
  const suffix = `${__VU}-${__ITER}-${Date.now()}`;

  const sentinelId = findOrCreateVariousArtists();

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  const createdGroup = invoke(
    `${GROUP}/CreateGroup`,
    {
      group: {
        libraryEntryId: sentinelId,
        title: `Late Night Radio, Vol. 3 (${suffix})`,
        overview: 'A various-artists late-night compilation.',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateGroup(compilation)'
  );
  const groupId = createdGroup.json('group.id');

  const createdRelease = invoke(
    `${MUSIC_RELEASE}/CreateMusicRelease`,
    {
      musicRelease: {
        groupId: groupId,
        libraryEntryId: sentinelId,
        title: `Late Night Radio, Vol. 3 (${suffix})`,
        status: 'RELEASE_STATUS_STUB',
        mbid: `k6-va-http-mbid-${suffix}`,
        barcode: `k6-va-http-barcode-${suffix}`,
      },
    },
    'CreateMusicRelease(compilation)'
  );
  const releaseId = createdRelease.json('musicRelease.id');

  // Two Persons, two Tracks, each Track credited to a different Person via
  // ItemPerson{Role: artist, CreditedAs}.
  const createdPersonA = invoke(
    `${PERSON}/CreatePerson`,
    { person: { name: 'Reiko Amano', gender: 'GENDER_UNKNOWN', monitorMode: 'MONITOR_MODE_NONE' } },
    'CreatePerson(A)'
  );
  const personAId = createdPersonA.json('person.id');

  const createdPersonB = invoke(
    `${PERSON}/CreatePerson`,
    { person: { name: 'Julius Ferro', gender: 'GENDER_UNKNOWN', monitorMode: 'MONITOR_MODE_NONE' } },
    'CreatePerson(B)'
  );
  const personBId = createdPersonB.json('person.id');

  const createdTrackA = invoke(
    `${ITEM}/CreateItem`,
    {
      item: {
        contentType: 'music',
        libraryEntryId: sentinelId,
        groupId: groupId,
        title: 'Neon Static',
        status: 'ITEM_STATUS_IMPORTED',
        sequence: '1',
        metadata: { release_id: releaseId },
      },
    },
    'CreateItem(trackA)'
  );
  const trackAId = createdTrackA.json('item.id');

  const createdTrackB = invoke(
    `${ITEM}/CreateItem`,
    {
      item: {
        contentType: 'music',
        libraryEntryId: sentinelId,
        groupId: groupId,
        title: 'Midnight Frequency',
        status: 'ITEM_STATUS_IMPORTED',
        sequence: '2',
        metadata: { release_id: releaseId },
      },
    },
    'CreateItem(trackB)'
  );
  const trackBId = createdTrackB.json('item.id');

  invoke(
    `${ITEM_PERSON}/CreateItemPerson`,
    { itemPerson: { itemId: trackAId, personId: personAId, role: 'artist', creditedAs: 'Reiko Amano' } },
    'CreateItemPerson(trackA, personA)'
  );
  invoke(
    `${ITEM_PERSON}/CreateItemPerson`,
    { itemPerson: { itemId: trackBId, personId: personBId, role: 'artist', creditedAs: 'Julius Ferro' } },
    'CreateItemPerson(trackB, personB)'
  );

  // "Everything featuring Person X": ListItemPeople filtered by PersonID
  // must return exactly that person's track, not the other one.
  let res = invoke(`${ITEM_PERSON}/ListItemPeople`, { personId: personAId, pageSize: 10 }, 'ListItemPeople(personA)');
  check(res, {
    "personA's credits include trackA": (r) => (r.json('itemPeople') || []).some((ip) => ip.itemId === trackAId),
    "personA's credits exclude trackB": (r) => !(r.json('itemPeople') || []).some((ip) => ip.itemId === trackBId),
  });

  res = invoke(`${ITEM_PERSON}/ListItemPeople`, { personId: personBId, pageSize: 10 }, 'ListItemPeople(personB)');
  check(res, {
    "personB's credits include trackB": (r) => (r.json('itemPeople') || []).some((ip) => ip.itemId === trackBId),
    "personB's credits exclude trackA": (r) => !(r.json('itemPeople') || []).some((ip) => ip.itemId === trackAId),
  });

  // Teardown, reverse order. The sentinel LibraryEntry/ExternalID are
  // shared, idempotent infrastructure and are deliberately not deleted.
  invoke(`${ITEM_PERSON}/DeleteItemPerson`, { itemId: trackBId, personId: personBId, role: 'artist' }, 'DeleteItemPerson(trackB, personB)');
  invoke(`${ITEM_PERSON}/DeleteItemPerson`, { itemId: trackAId, personId: personAId, role: 'artist' }, 'DeleteItemPerson(trackA, personA)');
  invoke(`${PERSON}/DeletePerson`, { id: personBId }, 'DeletePerson(B)');
  invoke(`${PERSON}/DeletePerson`, { id: personAId }, 'DeletePerson(A)');
  invoke(`${ITEM}/DeleteItem`, { id: trackBId }, 'DeleteItem(trackB)');
  invoke(`${ITEM}/DeleteItem`, { id: trackAId }, 'DeleteItem(trackA)');
  invoke(`${MUSIC_RELEASE}/DeleteMusicRelease`, { id: releaseId }, 'DeleteMusicRelease');
  invoke(`${GROUP}/DeleteGroup`, { id: groupId }, 'DeleteGroup(compilation)');
};
