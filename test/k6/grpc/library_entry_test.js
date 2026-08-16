// k6 gRPC suite for LibraryEntryService. See test/k6/grpc/person_test.js
// for the pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/library_entry.proto', 'purser/domain/v1/group.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // ids are server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  let res = invoke('purser.domain.v1.LibraryEntryService/CreateLibraryEntry', {
    libraryEntry: { contentType: 'adult', kind: 'studio', name: 'K6 gRPC Studio', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, {
    'CreateLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateLibraryEntry returns an id': (r) => r && r.message && r.message.libraryEntry && !!r.message.libraryEntry.id,
  });
  const id = res.message.libraryEntry.id;

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: id });
  check(res, {
    'GetLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetLibraryEntry returns the created name': (r) => r && r.message && r.message.libraryEntry && r.message.libraryEntry.name === 'K6 gRPC Studio',
  });

  res = invoke('purser.domain.v1.LibraryEntryService/UpdateLibraryEntry', {
    libraryEntry: { id: id, name: 'K6 gRPC Studio Updated' },
    updateMask: 'name',
  });
  check(res, {
    'UpdateLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateLibraryEntry applied the field-masked name': (r) => r && r.message && r.message.libraryEntry && r.message.libraryEntry.name === 'K6 gRPC Studio Updated',
  });

  res = invoke('purser.domain.v1.LibraryEntryService/ListLibraryEntries', { pageSize: 10 });
  check(res, {
    'ListLibraryEntries status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListLibraryEntries includes the created entry': (r) =>
      r && r.message && r.message.libraryEntries && r.message.libraryEntries.some((e) => e.id === id),
  });

  res = invoke('purser.domain.v1.LibraryEntryService/ListLibraryEntries', { kind: 'studio', pageSize: 10 });
  check(res, {
    'ListLibraryEntries filtered by kind status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListLibraryEntries filtered by kind includes the created entry': (r) =>
      r && r.message && r.message.libraryEntries && r.message.libraryEntries.some((e) => e.id === id),
  });

  res = invoke('purser.domain.v1.LibraryEntryService/ListLibraryEntries', { kind: 'network', pageSize: 10 });
  check(res, {
    'ListLibraryEntries filtered by a non-matching kind excludes the created entry': (r) =>
      r && r.message && !(r.message.libraryEntries || []).some((e) => e.id === id),
  });

  // Deletion-impact + Unlink: a child LibraryEntry is a non-blocking
  // referrer — deleting the parent without cascade detaches the child
  // (blanks its parentId) rather than deleting it or failing.
  res = invoke('purser.domain.v1.LibraryEntryService/CreateLibraryEntry', {
    libraryEntry: { contentType: 'adult', kind: 'studio', name: 'K6 gRPC Child Studio', parentId: id, monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, {
    'CreateLibraryEntry (child) status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateLibraryEntry (child) returns an id': (r) => r && r.message && r.message.libraryEntry && !!r.message.libraryEntry.id,
  });
  const childId = res.message.libraryEntry.id;

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntryDeletionImpact', { id: id });
  check(res, {
    'GetLibraryEntryDeletionImpact status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetLibraryEntryDeletionImpact reports the child as non-blocking': (r) =>
      r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'library_entry_child' && i.count === 1 && !i.blocking),
  });

  res = invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: id });
  check(res, {
    'DeleteLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: id });
  check(res, {
    'GetLibraryEntry after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound,
  });

  // The child must still exist, just detached (parentId cleared) — a
  // non-blocking referrer is unlinked, not deleted, on a plain Delete.
  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: childId });
  check(res, {
    'GetLibraryEntry after parent Delete still finds the child (detached, not deleted)': (r) => r && r.status === grpc.StatusOK,
    'GetLibraryEntry after parent Delete shows parentId cleared': (r) => r && r.message && r.message.libraryEntry && r.message.libraryEntry.parentId === '',
  });

  // Blocking + cascade: a Group is a structural referrer (required FK) —
  // deleting its LibraryEntry without cascade must fail, and only
  // cascade=true removes both.
  res = invoke('purser.domain.v1.GroupService/CreateGroup', {
    group: { libraryEntryId: childId, title: 'K6 gRPC LibraryEntry Deletion Group', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, {
    'CreateGroup status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateGroup returns an id': (r) => r && r.message && r.message.group && !!r.message.group.id,
  });
  const groupId = res.message.group.id;

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntryDeletionImpact', { id: childId });
  check(res, {
    'GetLibraryEntryDeletionImpact reports the group as blocking': (r) =>
      r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'group' && i.count === 1 && i.blocking),
  });

  res = invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: childId });
  check(res, {
    'DeleteLibraryEntry without cascade is FailedPrecondition when a Group exists': (r) => r && r.status === grpc.StatusFailedPrecondition,
  });

  res = invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: childId, cascade: true });
  check(res, {
    'DeleteLibraryEntry with cascade status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: childId });
  check(res, { 'GetLibraryEntry after cascade Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: groupId });
  check(res, { 'GetGroup after cascade Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  // BulkDeleteLibraryEntries: the "delete these 12 duplicate studios" use
  // case — see docs/adr/0016-bulk-operations.md.
  const bulkIds = [];
  for (let i = 0; i < 3; i++) {
    res = invoke('purser.domain.v1.LibraryEntryService/CreateLibraryEntry', {
      libraryEntry: { contentType: 'adult', kind: 'studio', name: 'K6 Bulk Entry', monitorMode: 'MONITOR_MODE_NONE' },
    });
    check(res, {
      'setup: CreateLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
      'setup: CreateLibraryEntry returns an id': (r) => r && r.message && r.message.libraryEntry && !!r.message.libraryEntry.id,
    });
    bulkIds.push(res.message.libraryEntry.id);
  }
  const [bulkId1, bulkId2, bulkId3] = bulkIds;

  res = invoke('purser.domain.v1.LibraryEntryService/BulkDeleteLibraryEntries', { ids: [bulkId1, bulkId2] });
  check(res, { 'BulkDeleteLibraryEntries status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: bulkId1 });
  check(res, { 'GetLibraryEntry for bulkId1 after BulkDeleteLibraryEntries is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: bulkId2 });
  check(res, { 'GetLibraryEntry for bulkId2 after BulkDeleteLibraryEntries is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: bulkId3 });
  check(res, { 'GetLibraryEntry for bulkId3 (not in the batch) still exists': (r) => r && r.status === grpc.StatusOK });

  // All-or-nothing: a batch with one missing id must fail entirely — the
  // still-existing bulkId3 must not be removed either.
  res = invoke('purser.domain.v1.LibraryEntryService/BulkDeleteLibraryEntries', { ids: [bulkId3, 'k6-grpc-entry-missing'] });
  check(res, { 'BulkDeleteLibraryEntries with a missing id is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: bulkId3 });
  check(res, { 'GetLibraryEntry for bulkId3 after failed batch still exists (rolled back)': (r) => r && r.status === grpc.StatusOK });

  // All-or-nothing precondition: bulkId3 (clean) plus a row with a Group
  // under it and cascade=false must fail the whole batch, per issue #654's
  // acceptance criterion — bulkId3 must not be removed either, even
  // though it's otherwise perfectly deletable on its own.
  res = invoke('purser.domain.v1.GroupService/CreateGroup', {
    group: { libraryEntryId: bulkId3, title: 'K6 Bulk Entry Blocking Group', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, {
    'setup: CreateGroup (blocking) status is OK': (r) => r && r.status === grpc.StatusOK,
    'setup: CreateGroup (blocking) returns an id': (r) => r && r.message && r.message.group && !!r.message.group.id,
  });
  const blockingGroupId = res.message.group.id;

  res = invoke('purser.domain.v1.LibraryEntryService/BulkDeleteLibraryEntries', { ids: [bulkId3] });
  check(res, {
    'BulkDeleteLibraryEntries without cascade is FailedPrecondition when a Group exists': (r) => r && r.status === grpc.StatusFailedPrecondition,
  });
  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: bulkId3 });
  check(res, { 'GetLibraryEntry for bulkId3 after blocked batch still exists': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.LibraryEntryService/BulkDeleteLibraryEntries', { ids: [bulkId3], cascade: true });
  check(res, { 'BulkDeleteLibraryEntries with cascade status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: bulkId3 });
  check(res, { 'GetLibraryEntry for bulkId3 after cascade batch is NotFound': (r) => r && r.status === grpc.StatusNotFound });
  res = invoke('purser.domain.v1.GroupService/GetGroup', { id: blockingGroupId });
  check(res, { 'GetGroup for the blocking group after cascade batch is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
