// k6 HTTP/JSON suite for AfterDark's BrowseService — the composing service
// that answers cross-entity reads (scenes in a network, performers for a
// scene/studio/network) no single entity's own service can answer alone.
// See docs/adr/0015-deletion-impact-and-composing-services.md. Unlike a
// single-entity suite, this script must first create fixtures across
// several other services (LibraryEntry, Person, PerformerProfile, Item,
// ItemPerson) before BrowseService has anything to compose over. See
// test/k6/grpc/afterdark_browse_test.js for the same fixture graph.
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const LIBRARY_ENTRY = `${BASE_URL}/purser.domain.v1.LibraryEntryService`;
const PERSON = `${BASE_URL}/purser.domain.v1.PersonService`;
const ITEM = `${BASE_URL}/purser.domain.v1.ItemService`;
const ITEM_PERSON = `${BASE_URL}/purser.domain.v1.ItemPersonService`;
const PERFORMER_PROFILE = `${BASE_URL}/purser.afterdark.v1.PerformerProfileService`;
const BROWSE = `${BASE_URL}/purser.afterdark.v1.BrowseService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const networkId = `k6-http-browse-network-${suffix}`;
  const studioId = `k6-http-browse-studio-${suffix}`;
  const personId = `k6-http-browse-person-${suffix}`;
  const sceneId = `k6-http-browse-scene-${suffix}`;

  let res = invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
    JSON.stringify({ libraryEntry: { id: networkId, contentType: 'adult', kind: 'network', name: 'K6 Browse Network', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreateLibraryEntry(network) status is 200': (r) => r.status === 200 });

  res = invoke(
    `${LIBRARY_ENTRY}/CreateLibraryEntry`,
    JSON.stringify({
      libraryEntry: { id: studioId, contentType: 'adult', kind: 'studio', parentId: networkId, name: 'K6 Browse Studio', monitorMode: 'MONITOR_MODE_NONE' },
    }),
    HEADERS
  );
  check(res, { 'CreateLibraryEntry(studio) status is 200': (r) => r.status === 200 });

  res = invoke(
    `${PERSON}/CreatePerson`,
    JSON.stringify({ person: { id: personId, name: 'K6 Browse Performer', gender: 'GENDER_UNKNOWN', monitorMode: 'MONITOR_MODE_NONE' } }),
    HEADERS
  );
  check(res, { 'CreatePerson status is 200': (r) => r.status === 200 });

  res = invoke(`${PERFORMER_PROFILE}/CreatePerformerProfile`, JSON.stringify({ performerProfile: { personId: personId } }), HEADERS);
  check(res, { 'CreatePerformerProfile status is 200': (r) => r.status === 200 });

  res = invoke(
    `${ITEM}/CreateItem`,
    JSON.stringify({ item: { id: sceneId, contentType: 'adult', libraryEntryId: studioId, title: 'K6 Browse Scene', status: 'ITEM_STATUS_WANTED' } }),
    HEADERS
  );
  check(res, { 'CreateItem status is 200': (r) => r.status === 200 });

  res = invoke(
    `${ITEM_PERSON}/CreateItemPerson`,
    JSON.stringify({ itemPerson: { itemId: sceneId, personId: personId, role: 'performer' } }),
    HEADERS
  );
  check(res, { 'CreateItemPerson status is 200': (r) => r.status === 200 });

  res = invoke(`${BROWSE}/ListScenesInNetwork`, JSON.stringify({ networkId: networkId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListScenesInNetwork status is 200': (r) => r.status === 200,
    'ListScenesInNetwork includes the created scene': (r) => (r.json('scenes') || []).some((s) => s.id === sceneId),
  });

  res = invoke(`${BROWSE}/ListScenesForPerformer`, JSON.stringify({ personId: personId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListScenesForPerformer status is 200': (r) => r.status === 200,
    'ListScenesForPerformer includes the created scene': (r) => (r.json('scenes') || []).some((s) => s.id === sceneId),
  });

  res = invoke(`${BROWSE}/ListPerformersForScene`, JSON.stringify({ itemId: sceneId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListPerformersForScene status is 200': (r) => r.status === 200,
    'ListPerformersForScene includes the created performer': (r) => (r.json('performers') || []).some((p) => p.person && p.person.id === personId),
  });

  res = invoke(`${BROWSE}/ListPerformersForStudio`, JSON.stringify({ libraryEntryId: studioId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListPerformersForStudio status is 200': (r) => r.status === 200,
    'ListPerformersForStudio includes the created performer': (r) => (r.json('performers') || []).some((p) => p.person && p.person.id === personId),
  });

  res = invoke(`${BROWSE}/ListPerformersForNetwork`, JSON.stringify({ networkId: networkId, pageSize: 10 }), HEADERS);
  check(res, {
    'ListPerformersForNetwork status is 200': (r) => r.status === 200,
    'ListPerformersForNetwork includes the created performer': (r) => (r.json('performers') || []).some((p) => p.person && p.person.id === personId),
  });

  res = invoke(`${BROWSE}/ListPerformers`, JSON.stringify({ pageSize: 10 }), HEADERS);
  check(res, {
    'ListPerformers status is 200': (r) => r.status === 200,
    'ListPerformers includes the created performer': (r) => (r.json('performers') || []).some((p) => p.person && p.person.id === personId),
  });

  res = invoke(`${ITEM_PERSON}/DeleteItemPerson`, JSON.stringify({ itemId: sceneId, personId: personId, role: 'performer' }), HEADERS);
  check(res, { 'DeleteItemPerson status is 200': (r) => r.status === 200 });

  res = invoke(`${ITEM}/DeleteItem`, JSON.stringify({ id: sceneId }), HEADERS);
  check(res, { 'DeleteItem status is 200': (r) => r.status === 200 });

  res = invoke(`${PERFORMER_PROFILE}/DeletePerformerProfile`, JSON.stringify({ personId: personId }), HEADERS);
  check(res, { 'DeletePerformerProfile status is 200': (r) => r.status === 200 });

  res = invoke(`${PERSON}/DeletePerson`, JSON.stringify({ id: personId }), HEADERS);
  check(res, { 'DeletePerson status is 200': (r) => r.status === 200 });

  res = invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, JSON.stringify({ id: studioId }), HEADERS);
  check(res, { 'DeleteLibraryEntry(studio) status is 200': (r) => r.status === 200 });

  res = invoke(`${LIBRARY_ENTRY}/DeleteLibraryEntry`, JSON.stringify({ id: networkId }), HEADERS);
  check(res, { 'DeleteLibraryEntry(network) status is 200': (r) => r.status === 200 });
};
