// k6 gRPC suite for PersonService — native gRPC wire protocol, loading the
// .proto directly (no server reflection dependency). Run from the repo
// root via `make k6-grpc` (or `k6 run test/k6/grpc/person_test.js`)
// against a running `purser serve` (PURSER_GRPC_ADDR, default
// localhost:7474). See docs/adr/0011-api-design.md.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/person.proto', 'purser/afterdark/v1/performer_profile.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;

  let res = client.invoke('purser.domain.v1.PersonService/CreatePerson', {
    person: {
      id: id,
      name: 'K6 gRPC Person',
      gender: 'GENDER_UNKNOWN',
      monitorMode: 'MONITOR_MODE_NONE',
    },
  });
  check(res, {
    'CreatePerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreatePerson returns the id': (r) => r && r.message && r.message.person && r.message.person.id === id,
  });

  res = client.invoke('purser.domain.v1.PersonService/GetPerson', { id: id });
  check(res, {
    'GetPerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetPerson returns the created name': (r) => r && r.message && r.message.person && r.message.person.name === 'K6 gRPC Person',
  });

  // google.protobuf.FieldMask's protojson mapping is a comma-joined
  // string of field paths, not {paths: [...]} — k6/net/grpc marshals
  // request objects the same way, so the object shape is a real error
  // here too, not just over HTTP/JSON.
  res = client.invoke('purser.domain.v1.PersonService/UpdatePerson', {
    person: { id: id, name: 'K6 gRPC Person Updated' },
    updateMask: 'name',
  });
  check(res, {
    'UpdatePerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdatePerson applied the field-masked name': (r) => r && r.message && r.message.person && r.message.person.name === 'K6 gRPC Person Updated',
  });

  res = client.invoke('purser.domain.v1.PersonService/ListPeople', { pageSize: 10 });
  check(res, {
    'ListPeople status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListPeople includes the created person': (r) =>
      r && r.message && r.message.people && r.message.people.some((p) => p.id === id),
  });

  res = client.invoke('purser.domain.v1.PersonService/GetPerson', { id: 'missing-' + id });
  check(res, {
    'GetPerson on a missing id is NotFound': (r) => r && r.status === grpc.StatusNotFound,
  });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/CreatePerformerProfile', { performerProfile: { personId: id } });
  check(res, { 'CreatePerformerProfile status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.PersonService/GetPersonDeletionImpact', { id: id });
  check(res, {
    'GetPersonDeletionImpact status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetPersonDeletionImpact reports the performer profile': (r) =>
      r && r.message && r.message.impacts && r.message.impacts.some((i) => i.kind === 'performer_profile' && i.count === 1),
  });

  res = client.invoke('purser.domain.v1.PersonService/DeletePerson', { id: id });
  check(res, {
    'DeletePerson status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  res = client.invoke('purser.domain.v1.PersonService/GetPerson', { id: id });
  check(res, {
    'GetPerson after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound,
  });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/GetPerformerProfile', { personId: id });
  check(res, { 'GetPerformerProfile after Person Delete is NotFound (unlinked)': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
