// k6 flow: "import a scene" — the composite task a scraper/importer or a
// UI's "add scene" form runs when scene metadata resolves to a studio, a
// network, and several performers, some of which may be new to the
// library. This is the realistic end-to-end shape the other two flows
// (create_performer_test.js, create_studio_test.js) are each one piece of:
// here everything is assembled in the sequence an import actually happens
// in, then verified through every read the milestone requires (scenes in
// a network, scenes for a studio, scenes for a performer, performers for
// a scene/studio/network).
//
// Fixture data is fictionalized but shape-accurate: pulled from live
// StashDB (queryStudios/queryPerformers/queryScenes/queryTags) and
// ThePornDB (/scenes, /performers) during design — real hierarchy shape
// (Network -> Studio, e.g. "Soft On Demand" parenting "SOD Create"/
// "IEnergy"), real performer attribute fields (gender, ethnicity, hair/eye
// color, height, breast_type, career_start_year on StashDB; measurements/
// cupsize/tattoos/piercings under ThePornDB's extras), real scene fields
// (title, details, date, duration, studio, credited performers via an
// "as" name distinct from the canonical one, granular tags with a
// category+group taxonomy like "Themes"/SCENE or "Age Group"/PEOPLE) —
// with every actual name, title, and attribute value invented rather than
// reused from any real record. The cast intentionally spans genders
// (female, male, transgender female) the same way StashDB's own
// GenderEnum — which internal/domain.Gender mirrors exactly — does.
import grpc from 'k6/net/grpc';
import { check, fail } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/item.proto',
  'purser/domain/v1/person.proto',
  'purser/domain/v1/item_person.proto',
  'purser/domain/v1/tag.proto',
  'purser/domain/v1/tag_assignment.proto',
  'purser/afterdark/v1/performer_profile.proto',
  'purser/afterdark/v1/browse.proto'
);

function invoke(method, request, label) {
  const res = client.invoke(method, request);
  const ok = check(res, { [`${label} status is OK`]: (r) => r && r.status === grpc.StatusOK });
  if (!ok) {
    fail(`${label} failed: ${res && res.status} ${res && res.error && res.error.message}`);
  }
  return res.message;
}

function createPerformer(id, person, profile) {
  invoke('purser.domain.v1.PersonService/CreatePerson', { person: Object.assign({ id: id, monitorMode: 'MONITOR_MODE_NONE' }, person) }, `CreatePerson(${person.name})`);
  invoke(
    'purser.afterdark.v1.PerformerProfileService/CreatePerformerProfile',
    { performerProfile: Object.assign({ personId: id }, profile) },
    `CreatePerformerProfile(${person.name})`
  );
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const networkId = `k6-flow-import-network-${suffix}`;
  const studioId = `k6-flow-import-studio-${suffix}`;
  const sceneId = `k6-flow-import-scene-${suffix}`;
  const harlowId = `k6-flow-import-harlow-${suffix}`;
  const darioId = `k6-flow-import-dario-${suffix}`;
  const mikaId = `k6-flow-import-mika-${suffix}`;
  const biancaId = `k6-flow-import-bianca-${suffix}`;
  const performerIds = [harlowId, darioId, mikaId, biancaId];
  const genreTagId = `k6-flow-import-genre-tag-${suffix}`;
  const settingTagId = `k6-flow-import-setting-tag-${suffix}`;

  // 1. Resolve/create the network and studio the scene's site metadata
  // points at.
  invoke(
    'purser.domain.v1.LibraryEntryService/CreateLibraryEntry',
    { libraryEntry: { id: networkId, contentType: 'adult', kind: 'network', name: 'Twilight Media (Network)', monitorMode: 'MONITOR_MODE_ALL' } },
    'CreateLibraryEntry(network)'
  );
  invoke(
    'purser.domain.v1.LibraryEntryService/CreateLibraryEntry',
    {
      libraryEntry: {
        id: studioId,
        contentType: 'adult',
        kind: 'studio',
        parentId: networkId,
        name: 'Twilight Media x Velvet Hour',
        monitorMode: 'MONITOR_MODE_ALL',
      },
    },
    'CreateLibraryEntry(studio)'
  );

  // 2. Resolve/create the performers the scene's metadata credits — a mix
  // of genders and attribute shapes drawn from both providers' performer
  // schemas.
  createPerformer(
    harlowId,
    { name: 'Harlow Vance', sortName: 'Vance, Harlow', gender: 'GENDER_FEMALE', nationality: 'American' },
    { cupSize: 'C', bandSize: '34', breastType: 'NATURAL', careerStartYear: 2021 }
  );
  createPerformer(
    darioId,
    { name: 'Dario Cole', sortName: 'Cole, Dario', gender: 'GENDER_MALE', nationality: 'American' },
    { breastType: 'NA', careerStartYear: 2019 }
  );
  createPerformer(
    mikaId,
    { name: 'Mika Delgado', sortName: 'Delgado, Mika', gender: 'GENDER_FEMALE', nationality: 'Canadian' },
    { cupSize: 'D', bandSize: '32', breastType: 'FAKE', careerStartYear: 2020 }
  );
  createPerformer(
    biancaId,
    { name: 'Bianca Storm', sortName: 'Storm, Bianca', gender: 'GENDER_TRANSGENDER_FEMALE', pronouns: 'she/her', nationality: 'Brazilian' },
    { cupSize: 'C', bandSize: '34', breastType: 'FAKE', careerStartYear: 2022 }
  );

  // 3. Create the scene under the resolved studio.
  invoke(
    'purser.domain.v1.ItemService/CreateItem',
    {
      item: {
        id: sceneId,
        contentType: 'adult',
        libraryEntryId: studioId,
        title: 'Velvet Hour: After Party',
        overview: 'Strangers reconnect at a rooftop after-party once the last guests have gone home.',
        runtimeSeconds: 2700,
        status: 'ITEM_STATUS_IMPORTED',
      },
    },
    'CreateItem'
  );

  // 4. Link each resolved performer to the scene, using a credited/stage
  // name distinct from the canonical Person name for some of them — the
  // same "as" mechanism StashDB's performers[].as field models.
  invoke(
    'purser.domain.v1.ItemPersonService/CreateItemPerson',
    { itemPerson: { itemId: sceneId, personId: harlowId, role: 'performer', creditedAs: 'Harlow V.' } },
    'CreateItemPerson(Harlow)'
  );
  invoke(
    'purser.domain.v1.ItemPersonService/CreateItemPerson',
    { itemPerson: { itemId: sceneId, personId: darioId, role: 'performer', creditedAs: 'Dario Cole' } },
    'CreateItemPerson(Dario)'
  );
  invoke(
    'purser.domain.v1.ItemPersonService/CreateItemPerson',
    { itemPerson: { itemId: sceneId, personId: mikaId, role: 'performer', creditedAs: 'Mika D.' } },
    'CreateItemPerson(Mika)'
  );
  invoke(
    'purser.domain.v1.ItemPersonService/CreateItemPerson',
    { itemPerson: { itemId: sceneId, personId: biancaId, role: 'performer', creditedAs: 'Bianca Storm' } },
    'CreateItemPerson(Bianca)'
  );

  // 5. Tag the scene with the genre/setting metadata the import resolved.
  invoke(
    'purser.domain.v1.TagService/CreateTag',
    { tag: { id: genreTagId, key: 'genre', value: 'Contemporary Romance', scope: 'TAG_SCOPE_METADATA', category: 'Themes' } },
    'CreateTag(genre)'
  );
  invoke(
    'purser.domain.v1.TagAssignmentService/CreateTagAssignment',
    { tagAssignment: { tagId: genreTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId } },
    'CreateTagAssignment(genre)'
  );
  invoke(
    'purser.domain.v1.TagService/CreateTag',
    { tag: { id: settingTagId, key: 'setting', value: 'Rooftop', scope: 'TAG_SCOPE_METADATA', category: 'Location' } },
    'CreateTag(setting)'
  );
  invoke(
    'purser.domain.v1.TagAssignmentService/CreateTagAssignment',
    { tagAssignment: { tagId: settingTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId } },
    'CreateTagAssignment(setting)'
  );

  // 6. Verify the full picture through every read the milestone requires.
  function hasAllPerformers(m) {
    const ids = new Set((m.performers || []).map((p) => p.person && p.person.id));
    return performerIds.every((id) => ids.has(id));
  }

  let msg = invoke('purser.domain.v1.LibraryEntryService/ListLibraryEntries', { kind: 'network', pageSize: 50 }, 'ListLibraryEntries(networks)');
  check(msg, { 'list networks includes the created network': (m) => (m.libraryEntries || []).some((e) => e.id === networkId) });

  msg = invoke(
    'purser.domain.v1.LibraryEntryService/ListLibraryEntries',
    { kind: 'studio', parentId: networkId, pageSize: 50 },
    'ListLibraryEntries(studios)'
  );
  check(msg, { 'list studios includes the created studio': (m) => (m.libraryEntries || []).some((e) => e.id === studioId) });

  msg = invoke('purser.afterdark.v1.BrowseService/ListPerformers', { pageSize: 50 }, 'ListPerformers');
  check(msg, { 'list performers includes all four imported performers': hasAllPerformers });

  msg = invoke('purser.afterdark.v1.BrowseService/ListScenesInNetwork', { networkId: networkId, pageSize: 50 }, 'ListScenesInNetwork');
  check(msg, { 'list scenes in network includes the imported scene': (m) => (m.scenes || []).some((s) => s.id === sceneId) });

  msg = invoke('purser.domain.v1.ItemService/ListItems', { libraryEntryId: studioId, contentType: 'adult', pageSize: 50 }, 'ListItems(scenes for studio)');
  check(msg, { 'list scenes for studio includes the imported scene': (m) => (m.items || []).some((i) => i.id === sceneId) });

  msg = invoke('purser.afterdark.v1.BrowseService/ListScenesForPerformer', { personId: biancaId, pageSize: 50 }, 'ListScenesForPerformer');
  check(msg, { 'list scenes for performer includes the imported scene': (m) => (m.scenes || []).some((s) => s.id === sceneId) });

  msg = invoke('purser.afterdark.v1.BrowseService/ListPerformersForScene', { itemId: sceneId, pageSize: 50 }, 'ListPerformersForScene');
  check(msg, { 'list performers for scene includes all four imported performers': hasAllPerformers });

  msg = invoke('purser.afterdark.v1.BrowseService/ListPerformersForStudio', { libraryEntryId: studioId, pageSize: 50 }, 'ListPerformersForStudio');
  check(msg, { 'list performers for studio includes all four imported performers': hasAllPerformers });

  msg = invoke('purser.afterdark.v1.BrowseService/ListPerformersForNetwork', { networkId: networkId, pageSize: 50 }, 'ListPerformersForNetwork');
  check(msg, { 'list performers for network includes all four imported performers': hasAllPerformers });

  // Teardown, reverse dependency order.
  invoke(
    'purser.domain.v1.TagAssignmentService/DeleteTagAssignment',
    { tagId: settingTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId },
    'DeleteTagAssignment(setting)'
  );
  invoke('purser.domain.v1.TagService/DeleteTag', { id: settingTagId }, 'DeleteTag(setting)');
  invoke(
    'purser.domain.v1.TagAssignmentService/DeleteTagAssignment',
    { tagId: genreTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId },
    'DeleteTagAssignment(genre)'
  );
  invoke('purser.domain.v1.TagService/DeleteTag', { id: genreTagId }, 'DeleteTag(genre)');

  for (const id of performerIds) {
    invoke('purser.domain.v1.ItemPersonService/DeleteItemPerson', { itemId: sceneId, personId: id, role: 'performer' }, `DeleteItemPerson(${id})`);
  }

  invoke('purser.domain.v1.ItemService/DeleteItem', { id: sceneId }, 'DeleteItem');

  for (const id of performerIds) {
    invoke('purser.afterdark.v1.PerformerProfileService/DeletePerformerProfile', { personId: id }, `DeletePerformerProfile(${id})`);
    invoke('purser.domain.v1.PersonService/DeletePerson', { id: id }, `DeletePerson(${id})`);
  }

  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: studioId }, 'DeleteLibraryEntry(studio)');
  invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: networkId }, 'DeleteLibraryEntry(network)');

  client.close();
};
