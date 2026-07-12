// k6 gRPC suite for LibraryEntryService. See test/k6/grpc/person_test.js
// for the pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:8080';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/library_entry.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;

  let res = client.invoke('purser.domain.v1.LibraryEntryService/CreateLibraryEntry', {
    libraryEntry: { id: id, contentType: 'adult', kind: 'studio', name: 'K6 gRPC Studio', monitorMode: 'MONITOR_MODE_NONE' },
  });
  check(res, {
    'CreateLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateLibraryEntry returns the id': (r) => r && r.message && r.message.libraryEntry && r.message.libraryEntry.id === id,
  });

  res = client.invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: id });
  check(res, {
    'GetLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetLibraryEntry returns the created name': (r) => r && r.message && r.message.libraryEntry && r.message.libraryEntry.name === 'K6 gRPC Studio',
  });

  res = client.invoke('purser.domain.v1.LibraryEntryService/UpdateLibraryEntry', {
    libraryEntry: { id: id, name: 'K6 gRPC Studio Updated' },
    updateMask: 'name',
  });
  check(res, {
    'UpdateLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateLibraryEntry applied the field-masked name': (r) => r && r.message && r.message.libraryEntry && r.message.libraryEntry.name === 'K6 gRPC Studio Updated',
  });

  res = client.invoke('purser.domain.v1.LibraryEntryService/ListLibraryEntries', { pageSize: 10 });
  check(res, {
    'ListLibraryEntries status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListLibraryEntries includes the created entry': (r) =>
      r && r.message && r.message.libraryEntries && r.message.libraryEntries.some((e) => e.id === id),
  });

  res = client.invoke('purser.domain.v1.LibraryEntryService/DeleteLibraryEntry', { id: id });
  check(res, {
    'DeleteLibraryEntry status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  res = client.invoke('purser.domain.v1.LibraryEntryService/GetLibraryEntry', { id: id });
  check(res, {
    'GetLibraryEntry after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound,
  });

  client.close();
};
