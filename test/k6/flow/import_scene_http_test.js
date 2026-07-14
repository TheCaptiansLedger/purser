// k6 flow (HTTP/JSON twin of import_scene_test.js): "import a scene" — the
// composite task a scraper/importer or a UI's "add scene" form runs when
// scene metadata resolves to a studio, a network, and several performers,
// some of which may be new to the library.
//
// Same fixture data and cast as import_scene_test.js — fictionalized but
// shape-accurate, pulled from live StashDB/ThePornDB responses during
// design, spanning female/male/transgender-female performers the same way
// StashDB's own GenderEnum does. See that script's header for the sourcing
// note.
import http from 'k6/http';
import { check, fail } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
const PERSON = `${BASE_URL}/purser.domain.v1.PersonService`;
const ITEM_PERSON = `${BASE_URL}/purser.domain.v1.ItemPersonService`;
const TAG = `${BASE_URL}/purser.domain.v1.TagService`;
const TAG_ASSIGNMENT = `${BASE_URL}/purser.domain.v1.TagAssignmentService`;
const PERFORMER_PROFILE = `${BASE_URL}/purser.afterdark.v1.PerformerProfileService`;
const BROWSE = `${BASE_URL}/purser.afterdark.v1.BrowseService`;
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

function createPerformer(id, person, profile) {
  invoke(`${PERSON}/CreatePerson`, { person: Object.assign({ id: id, monitorMode: 'MONITOR_MODE_NONE' }, person) }, `CreatePerson(${person.name})`);
  invoke(
    `${PERFORMER_PROFILE}/CreatePerformerProfile`,
    { performerProfile: Object.assign({ personId: id }, profile) },
    `CreatePerformerProfile(${person.name})`
  );
}

export default () => {
  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const networkId = `k6-flow-http-import-network-${suffix}`;
  const studioId = `k6-flow-http-import-studio-${suffix}`;
  const sceneId = `k6-flow-http-import-scene-${suffix}`;
  const harlowId = `k6-flow-http-import-harlow-${suffix}`;
  const darioId = `k6-flow-http-import-dario-${suffix}`;
  const mikaId = `k6-flow-http-import-mika-${suffix}`;
  const biancaId = `k6-flow-http-import-bianca-${suffix}`;
  const performerIds = [harlowId, darioId, mikaId, biancaId];
  const genreTagId = `k6-flow-http-import-genre-tag-${suffix}`;
  const settingTagId = `k6-flow-http-import-setting-tag-${suffix}`;

  // 1. Resolve/create the network and studio the scene's site metadata
  // points at.
  invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
    { libraryEntry: { id: networkId, contentType: 'adult', kind: 'network', name: 'Twilight Media (Network)', monitorMode: 'MONITOR_MODE_ALL' } },
    'CreateLibraryEntry(network)'
  );
  invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
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

  // 2. Resolve/create the performers the scene's metadata credits.
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
    `${ITEM}/CreateItem`,
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

  // 4. Link each resolved performer to the scene.
  invoke(
    `${ITEM_PERSON}/CreateItemPerson`,
    { itemPerson: { itemId: sceneId, personId: harlowId, role: 'performer', creditedAs: 'Harlow V.' } },
    'CreateItemPerson(Harlow)'
  );
  invoke(
    `${ITEM_PERSON}/CreateItemPerson`,
    { itemPerson: { itemId: sceneId, personId: darioId, role: 'performer', creditedAs: 'Dario Cole' } },
    'CreateItemPerson(Dario)'
  );
  invoke(
    `${ITEM_PERSON}/CreateItemPerson`,
    { itemPerson: { itemId: sceneId, personId: mikaId, role: 'performer', creditedAs: 'Mika D.' } },
    'CreateItemPerson(Mika)'
  );
  invoke(
    `${ITEM_PERSON}/CreateItemPerson`,
    { itemPerson: { itemId: sceneId, personId: biancaId, role: 'performer', creditedAs: 'Bianca Storm' } },
    'CreateItemPerson(Bianca)'
  );

  // 5. Tag the scene with the genre/setting metadata the import resolved.
  // value is suffixed per-VU: CreateTag is get-or-create on (scope, key,
  // value) (docs/adr/0019), so a literal value would make concurrent VUs
  // share one tag and race on this flow's own teardown DeleteTag.
  invoke(
    `${TAG}/CreateTag`,
    { tag: { id: genreTagId, key: 'genre', value: `Contemporary Romance ${suffix}`, scope: 'TAG_SCOPE_METADATA', category: 'Themes' } },
    'CreateTag(genre)'
  );
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: genreTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId } },
    'CreateTagAssignment(genre)'
  );
  invoke(
    `${TAG}/CreateTag`,
    { tag: { id: settingTagId, key: 'setting', value: `Rooftop ${suffix}`, scope: 'TAG_SCOPE_METADATA', category: 'Location' } },
    'CreateTag(setting)'
  );
  invoke(
    `${TAG_ASSIGNMENT}/CreateTagAssignment`,
    { tagAssignment: { tagId: settingTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId } },
    'CreateTagAssignment(setting)'
  );

  // 6. Verify the full picture through every read the milestone requires.
  function hasAllPerformers(r) {
    const ids = new Set((r.json('performers') || []).map((p) => p.person && p.person.id));
    return performerIds.every((id) => ids.has(id));
  }

  let res = invoke(`${LIBRARY_ENTRY}/ListLibraryEntries`, { kind: 'network', pageSize: 50 }, 'ListLibraryEntries(networks)');
  check(res, { 'list networks includes the created network': (r) => (r.json('libraryEntries') || []).some((e) => e.id === networkId) });

  res = invoke(`${LIBRARY_ENTRY}/ListLibraryEntries`, { kind: 'studio', parentId: networkId, pageSize: 50 }, 'ListLibraryEntries(studios)');
  check(res, { 'list studios includes the created studio': (r) => (r.json('libraryEntries') || []).some((e) => e.id === studioId) });

  res = invoke(`${BROWSE}/ListPerformers`, { pageSize: 50 }, 'ListPerformers');
  check(res, { 'list performers includes all four imported performers': hasAllPerformers });

  res = invoke(`${BROWSE}/ListScenesInNetwork`, { networkId: networkId, pageSize: 50 }, 'ListScenesInNetwork');
  check(res, { 'list scenes in network includes the imported scene': (r) => (r.json('scenes') || []).some((s) => s.id === sceneId) });

  res = invoke(`${ITEM}/ListItems`, { libraryEntryId: studioId, contentType: 'adult', pageSize: 50 }, 'ListItems(scenes for studio)');
  check(res, { 'list scenes for studio includes the imported scene': (r) => (r.json('items') || []).some((i) => i.id === sceneId) });

  res = invoke(`${BROWSE}/ListScenesForPerformer`, { personId: biancaId, pageSize: 50 }, 'ListScenesForPerformer');
  check(res, { 'list scenes for performer includes the imported scene': (r) => (r.json('scenes') || []).some((s) => s.id === sceneId) });

  res = invoke(`${BROWSE}/ListPerformersForScene`, { itemId: sceneId, pageSize: 50 }, 'ListPerformersForScene');
  check(res, { 'list performers for scene includes all four imported performers': hasAllPerformers });

  res = invoke(`${BROWSE}/ListPerformersForStudio`, { libraryEntryId: studioId, pageSize: 50 }, 'ListPerformersForStudio');
  check(res, { 'list performers for studio includes all four imported performers': hasAllPerformers });

  res = invoke(`${BROWSE}/ListPerformersForNetwork`, { networkId: networkId, pageSize: 50 }, 'ListPerformersForNetwork');
  check(res, { 'list performers for network includes all four imported performers': hasAllPerformers });

  // Tags are never embedded on Item — TagAssignment is a separate
  // polymorphic join. Prove both scene tags round-trip in both
  // directions: "this scene's tags" (both genre and setting together) and
  // "everything tagged this tag" (browse-by-tag).
  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId, pageSize: 10 }, 'ListTagAssignments(by scene)');
  check(res, {
    "the scene's tags include both the genre and setting tags": (r) => {
      const ids = new Set((r.json('tagAssignments') || []).map((ta) => ta.tagId));
      return ids.has(genreTagId) && ids.has(settingTagId);
    },
  });

  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { tagId: genreTagId, pageSize: 10 }, 'ListTagAssignments(by genre tag)');
  check(res, { 'everything tagged the genre tag includes the imported scene': (r) => (r.json('tagAssignments') || []).some((ta) => ta.entityId === sceneId) });

  res = invoke(`${TAG_ASSIGNMENT}/ListTagAssignments`, { tagId: settingTagId, pageSize: 10 }, 'ListTagAssignments(by setting tag)');
  check(res, { 'everything tagged the setting tag includes the imported scene': (r) => (r.json('tagAssignments') || []).some((ta) => ta.entityId === sceneId) });

  // Teardown, reverse dependency order.
  invoke(`${TAG_ASSIGNMENT}/DeleteTagAssignment`, { tagId: settingTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId }, 'DeleteTagAssignment(setting)');
  invoke(`${TAG}/DeleteTag`, { id: settingTagId }, 'DeleteTag(setting)');
  invoke(`${TAG_ASSIGNMENT}/DeleteTagAssignment`, { tagId: genreTagId, entityType: 'ENTITY_TYPE_ITEM', entityId: sceneId }, 'DeleteTagAssignment(genre)');
  invoke(`${TAG}/DeleteTag`, { id: genreTagId }, 'DeleteTag(genre)');

  for (const id of performerIds) {
    invoke(`${ITEM_PERSON}/DeleteItemPerson`, { itemId: sceneId, personId: id, role: 'performer' }, `DeleteItemPerson(${id})`);
  }

  invoke(`${ITEM}/DeleteItem`, { id: sceneId }, 'DeleteItem');

  for (const id of performerIds) {
    invoke(`${PERFORMER_PROFILE}/DeletePerformerProfile`, { personId: id }, `DeletePerformerProfile(${id})`);
    invoke(`${PERSON}/DeletePerson`, { id: id }, `DeletePerson(${id})`);
  }

  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: studioId }, 'DeleteLibraryEntry(studio)');
  invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, { id: networkId }, 'DeleteLibraryEntry(network)');
};
