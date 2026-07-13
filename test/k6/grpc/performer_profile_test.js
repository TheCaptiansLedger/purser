// k6 gRPC suite for PerformerProfileService — AfterDark's only
// module-specific service, keyed by personId alone (no independent id).
// See test/k6/grpc/person_test.js for the general pattern.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/afterdark/v1/performer_profile.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const personId = `k6-grpc-performer-${__VU}-${__ITER}-${Date.now()}`;

  let res = client.invoke('purser.afterdark.v1.PerformerProfileService/CreatePerformerProfile', {
    performerProfile: { personId: personId, cupSize: '34', bandSize: 'C', careerStartYear: 2015 },
  });
  check(res, {
    'CreatePerformerProfile status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreatePerformerProfile returns the person id': (r) =>
      r && r.message && r.message.performerProfile && r.message.performerProfile.personId === personId,
  });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/GetPerformerProfile', { personId: personId });
  check(res, {
    'GetPerformerProfile status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetPerformerProfile returns the created cup size': (r) =>
      r && r.message && r.message.performerProfile && r.message.performerProfile.cupSize === '34',
  });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/UpdatePerformerProfile', {
    performerProfile: { personId: personId, cupSize: '36' },
    updateMask: 'cupSize',
  });
  check(res, {
    'UpdatePerformerProfile status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdatePerformerProfile applied the field-masked cup size': (r) =>
      r && r.message && r.message.performerProfile && r.message.performerProfile.cupSize === '36',
  });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/ListPerformerProfiles', { pageSize: 10 });
  check(res, {
    'ListPerformerProfiles status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListPerformerProfiles includes the created profile': (r) =>
      r && r.message && r.message.performerProfiles && r.message.performerProfiles.some((p) => p.personId === personId),
  });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/DeletePerformerProfile', { personId: personId });
  check(res, { 'DeletePerformerProfile status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.afterdark.v1.PerformerProfileService/GetPerformerProfile', { personId: personId });
  check(res, { 'GetPerformerProfile after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
