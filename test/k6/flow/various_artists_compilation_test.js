// k6 flow: "import a Various Artists compilation" — the task a UI/admin
// tool runs when a compilation's Release Group is owned by MusicBrainz's
// well-known "Various Artists" sentinel artist rather than a real band,
// per docs/adr/0021-music-domain-model.md's "Various Artists
// compilations" section: LibraryEntryID stays required, never nullable,
// on Group/MusicRelease, and per-track attribution lives entirely in
// ItemPerson{Role: artist, CreditedAs}. This flow proves "everything
// featuring Person X" (ItemPerson list filtered by PersonID) returns the
// correct track for each of two differently-credited artists on the same
// compilation.
//
// The sentinel LibraryEntry is get-or-created inline here, mirroring
// cmd/purser/seed_various_artists.go's idempotent find-or-create logic —
// `make k6-ci` runs against a fresh, unseeded hermetic store (see
// docs/adr/0022-k6-ci-enforcement.md), so this flow can't assume a
// separate seeding step already ran. The sentinel itself is left in
// place on teardown since it's shared, idempotent infrastructure, not a
// fixture owned by this flow.
//
// Fixture data is fictionalized but shape-accurate.
import grpc from 'k6/net/grpc';
import { check, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';
const VARIOUS_ARTISTS_MBID = '89ad4ac3-39f7-470e-963a-56509c546377';
const VARIOUS_ARTISTS_NAME = 'Various Artists';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/external_id.proto',
  'purser/domain/v1/group.proto',
  'purser/domain/v1/item.proto',
  'purser/domain/v1/person.proto',
  'purser/domain/v1/item_person.proto',
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

// ensureVariousArtistsExternalID is the same idempotency guard
// cmd/purser/seed_various_artists.go uses: Get first (a concurrent VU may
// have found the LibraryEntry via List a moment after its creator ran
// this same Create but before this VU's own attempt), and only Create on
// NotFound.
function ensureVariousArtistsExternalID(entryId) {
  const res = client.invoke('purser.domain.v1.ExternalIDService/GetExternalID', {
    entityType: 'ENTITY_TYPE_LIBRARY_ENTRY',
    entityId: entryId,
    source: 'mbz',
  });
  if (res.status === grpc.StatusOK) {
    return;
  }
  invoke(
    'purser.domain.v1.ExternalIDService/CreateExternalID',
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
      'purser.domain.v1.LibraryEntryService/ListLibraryEntries',
      { kind: 'artist', pageToken: pageToken, pageSize: 50 },
      'ListLibraryEntries(artist, sentinel lookup)'
    );
    const found = (listed.libraryEntries || []).find((e) => e.name === VARIOUS_ARTISTS_NAME);
    if (found) {
      ensureVariousArtistsExternalID(found.id);
      return found.id;
    }
    pageToken = listed.nextPageToken;
    if (!pageToken) {
      break;
    }
  }

  const created = invoke(
    'purser.domain.v1.LibraryEntryService/CreateLibraryEntry',
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
  const entryId = created.libraryEntry.id;

  ensureVariousArtistsExternalID(entryId);

  return entryId;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const suffix = `${__VU}-${__ITER}-${Date.now()}`;

  const sentinelId = findOrCreateVariousArtists();

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  const createdGroup = invoke(
    'purser.domain.v1.GroupService/CreateGroup',
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
  const groupId = createdGroup.group.id;

  const createdRelease = invoke(
    'purser.music.v1.MusicReleaseService/CreateMusicRelease',
    {
      musicRelease: {
        groupId: groupId,
        libraryEntryId: sentinelId,
        title: `Late Night Radio, Vol. 3 (${suffix})`,
        status: 'RELEASE_STATUS_STUB',
        mbid: `k6-va-mbid-${suffix}`,
        barcode: `k6-va-barcode-${suffix}`,
      },
    },
    'CreateMusicRelease(compilation)'
  );
  const releaseId = createdRelease.musicRelease.id;

  // Two Persons, two Tracks, each Track credited to a different Person via
  // ItemPerson{Role: artist, CreditedAs}.
  const createdPersonA = invoke(
    'purser.domain.v1.PersonService/CreatePerson',
    { person: { name: 'Reiko Amano', gender: 'GENDER_UNKNOWN', monitorMode: 'MONITOR_MODE_NONE' } },
    'CreatePerson(A)'
  );
  const personAId = createdPersonA.person.id;

  const createdPersonB = invoke(
    'purser.domain.v1.PersonService/CreatePerson',
    { person: { name: 'Julius Ferro', gender: 'GENDER_UNKNOWN', monitorMode: 'MONITOR_MODE_NONE' } },
    'CreatePerson(B)'
  );
  const personBId = createdPersonB.person.id;

  const createdTrackA = invoke(
    'purser.domain.v1.ItemService/CreateItem',
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
  const trackAId = createdTrackA.item.id;

  const createdTrackB = invoke(
    'purser.domain.v1.ItemService/CreateItem',
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
  const trackBId = createdTrackB.item.id;

  invoke(
    'purser.domain.v1.ItemPersonService/CreateItemPerson',
    { itemPerson: { itemId: trackAId, personId: personAId, role: 'artist', creditedAs: 'Reiko Amano' } },
    'CreateItemPerson(trackA, personA)'
  );
  invoke(
    'purser.domain.v1.ItemPersonService/CreateItemPerson',
    { itemPerson: { itemId: trackBId, personId: personBId, role: 'artist', creditedAs: 'Julius Ferro' } },
    'CreateItemPerson(trackB, personB)'
  );

  // "Everything featuring Person X": ListItemPeople filtered by PersonID
  // must return exactly that person's track, not the other one.
  const creditsForA = invoke('purser.domain.v1.ItemPersonService/ListItemPeople', { personId: personAId, pageSize: 10 }, 'ListItemPeople(personA)');
  check(creditsForA, {
    "personA's credits include trackA": (m) => m && m.itemPeople && m.itemPeople.some((ip) => ip.itemId === trackAId),
    "personA's credits exclude trackB": (m) => m && m.itemPeople && !m.itemPeople.some((ip) => ip.itemId === trackBId),
  });

  const creditsForB = invoke('purser.domain.v1.ItemPersonService/ListItemPeople', { personId: personBId, pageSize: 10 }, 'ListItemPeople(personB)');
  check(creditsForB, {
    "personB's credits include trackB": (m) => m && m.itemPeople && m.itemPeople.some((ip) => ip.itemId === trackBId),
    "personB's credits exclude trackA": (m) => m && m.itemPeople && !m.itemPeople.some((ip) => ip.itemId === trackAId),
  });

  // Teardown, reverse order. The sentinel LibraryEntry/ExternalID are
  // shared, idempotent infrastructure and are deliberately not deleted.
  invoke('purser.domain.v1.ItemPersonService/DeleteItemPerson', { itemId: trackBId, personId: personBId, role: 'artist' }, 'DeleteItemPerson(trackB, personB)');
  invoke('purser.domain.v1.ItemPersonService/DeleteItemPerson', { itemId: trackAId, personId: personAId, role: 'artist' }, 'DeleteItemPerson(trackA, personA)');
  invoke('purser.domain.v1.PersonService/DeletePerson', { id: personBId }, 'DeletePerson(B)');
  invoke('purser.domain.v1.PersonService/DeletePerson', { id: personAId }, 'DeletePerson(A)');
  invoke('purser.domain.v1.ItemService/DeleteItem', { id: trackBId }, 'DeleteItem(trackB)');
  invoke('purser.domain.v1.ItemService/DeleteItem', { id: trackAId }, 'DeleteItem(trackA)');
  invoke('purser.music.v1.MusicReleaseService/DeleteMusicRelease', { id: releaseId }, 'DeleteMusicRelease');
  invoke('purser.domain.v1.GroupService/DeleteGroup', { id: groupId }, 'DeleteGroup(compilation)');

  client.close();
};
