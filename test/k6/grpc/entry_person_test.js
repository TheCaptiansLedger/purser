// k6 gRPC suite for EntryPersonService — a join-shaped entity keyed by the
// composite (libraryEntryId, personId, role), not a single id. See
// test/k6/grpc/person_test.js for the general pattern.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/entry_person.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const libraryEntryId = `k6-grpc-entry-${__VU}-${__ITER}-${Date.now()}`;
  const personId = 'k6-person-1';
  const role = 'director';

  let res = client.invoke('purser.domain.v1.EntryPersonService/CreateEntryPerson', {
    entryPerson: { libraryEntryId: libraryEntryId, personId: personId, role: role, creditedAs: 'K6 Director' },
  });
  check(res, {
    'CreateEntryPerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateEntryPerson returns the role': (r) => r && r.message && r.message.entryPerson && r.message.entryPerson.role === role,
  });

  res = client.invoke('purser.domain.v1.EntryPersonService/GetEntryPerson', { libraryEntryId: libraryEntryId, personId: personId, role: role });
  check(res, {
    'GetEntryPerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetEntryPerson returns the created credit': (r) => r && r.message && r.message.entryPerson && r.message.entryPerson.creditedAs === 'K6 Director',
  });

  res = client.invoke('purser.domain.v1.EntryPersonService/UpdateEntryPerson', {
    entryPerson: { libraryEntryId: libraryEntryId, personId: personId, role: role, creditedAs: 'K6 Director Updated' },
    updateMask: 'creditedAs',
  });
  check(res, {
    'UpdateEntryPerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateEntryPerson applied the field-masked credit': (r) =>
      r && r.message && r.message.entryPerson && r.message.entryPerson.creditedAs === 'K6 Director Updated',
  });

  res = client.invoke('purser.domain.v1.EntryPersonService/ListEntryPeople', { libraryEntryId: libraryEntryId, pageSize: 10 });
  check(res, {
    'ListEntryPeople status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListEntryPeople includes the created credit': (r) =>
      r && r.message && r.message.entryPeople && r.message.entryPeople.some((ep) => ep.personId === personId && ep.role === role),
  });

  res = client.invoke('purser.domain.v1.EntryPersonService/DeleteEntryPerson', { libraryEntryId: libraryEntryId, personId: personId, role: role });
  check(res, { 'DeleteEntryPerson status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.EntryPersonService/GetEntryPerson', { libraryEntryId: libraryEntryId, personId: personId, role: role });
  check(res, { 'GetEntryPerson after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
