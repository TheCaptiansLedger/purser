// k6 gRPC suite for AfterDark's BrowseService — the composing service that
// answers cross-entity reads (scenes in a network, performers for a
// scene/studio/network) no single entity's own service can answer alone.
// See docs/adr/0015-deletion-impact-and-composing-services.md. Unlike a
// single-entity suite, this script must first create fixtures across
// several other services (LibraryEntry, Person, PerformerProfile, Item,
// ItemPerson) before BrowseService has anything to compose over.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(
  ['../../../proto'],
  'purser/domain/v1/library_entry.proto',
  'purser/domain/v1/item.proto',
  'purser/domain/v1/person.proto',
  'purser/domain/v1/item_person.proto',
  'purser/afterdark/v1/performer_profile.proto',
  'purser/afterdark/v1/browse.proto'
);

export default () => {
  client.connect(ADDR, { plaintext: true });

  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const networkId = `k6-grpc-browse-network-${suffix}`;
  const studioId = `k6-grpc-browse-studio-${suffix}`;
  const personId = `k6-grpc-browse-person-${suffix}`;
  const sceneId = `k6-grpc-browse-scene-${suffix}`;

  let res = client.invoke('purser.domain.v1.LibraryEntryService/CreateLibraryEntry', {
    libraryEntry: { id: networkId, contentType: 'adult', kind: 'network', name: 'K6 Browse Network', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, { 'CreateLibraryEntry(network) status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.LibraryEntryService/CreateLibraryEntry', {
    libraryEntry: { id: studioId, contentType: 'adult', kind: 'studio', parentId: networkId, name: 'K6 Browse Studio', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, { 'CreateLibraryEntry(studio) status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.PersonService/CreatePerson', {
    person: { id: personId, name: 'K6 Browse Performer', gender: 'GENDER_UNKNOWN', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, { 'CreatePerson status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/CreatePerformerProfile', { performerProfile: { personId: personId } });
  check(res, { 'CreatePerformerProfile status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemService/CreateItem', {
    item: { id: sceneId, contentType: 'adult', libraryEntryId: studioId, title: 'K6 Browse Scene', status: 'ITEM_STATUS_WANTED' },
  });
  check(res, { 'CreateItem status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemPersonService/CreateItemPerson', {
    itemPerson: { itemId: sceneId, personId: personId, role: 'performer' },
  });
  check(res, { 'CreateItemPerson status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.afterdark.v1.BrowseService/ListScenesInNetwork', { networkId: networkId, pageSize: 10 });
  check(res, {
    'ListScenesInNetwork status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListScenesInNetwork includes the created scene': (r) => r && r.message && r.message.scenes && r.message.scenes.some((s) => s.id === sceneId),
  });

  res = client.invoke('purser.afterdark.v1.BrowseService/ListScenesForPerformer', { personId: personId, pageSize: 10 });
  check(res, {
    'ListScenesForPerformer status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListScenesForPerformer includes the created scene': (r) => r && r.message && r.message.scenes && r.message.scenes.some((s) => s.id === sceneId),
  });

  res = client.invoke('purser.afterdark.v1.BrowseService/ListPerformersForScene', { itemId: sceneId, pageSize: 10 });
  check(res, {
    'ListPerformersForScene status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListPerformersForScene includes the created performer': (r) =>
      r && r.message && r.message.performers && r.message.performers.some((p) => p.person && p.person.id === personId),
  });

  res = client.invoke('purser.afterdark.v1.BrowseService/ListPerformersForStudio', { libraryEntryId: studioId, pageSize: 10 });
  check(res, {
    'ListPerformersForStudio status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListPerformersForStudio includes the created performer': (r) =>
      r && r.message && r.message.performers && r.message.performers.some((p) => p.person && p.person.id === personId),
  });

  res = client.invoke('purser.afterdark.v1.BrowseService/ListPerformersForNetwork', { networkId: networkId, pageSize: 10 });
  check(res, {
    'ListPerformersForNetwork status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListPerformersForNetwork includes the created performer': (r) =>
      r && r.message && r.message.performers && r.message.performers.some((p) => p.person && p.person.id === personId),
  });

  res = client.invoke('purser.afterdark.v1.BrowseService/ListPerformers', { pageSize: 10 });
  check(res, {
    'ListPerformers status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListPerformers includes the created performer': (r) =>
      r && r.message && r.message.performers && r.message.performers.some((p) => p.person && p.person.id === personId),
  });

  res = client.invoke('purser.domain.v1.ItemPersonService/DeleteItemPerson', { itemId: sceneId, personId: personId, role: 'performer' });
  check(res, { 'DeleteItemPerson status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemService/DeleteItem', { id: sceneId });
  check(res, { 'DeleteItem status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/DeletePerformerProfile', { personId: personId });
  check(res, { 'DeletePerformerProfile status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.PersonService/DeletePerson', { id: personId });
  check(res, { 'DeletePerson status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: studioId });
  check(res, { 'DeleteLibraryEntry(studio) status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: networkId });
  check(res, { 'DeleteLibraryEntry(network) status is OK': (r) => r && r.status === grpc.StatusOK });

  client.close();
};
