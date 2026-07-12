// k6 gRPC suite for TagService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:8080';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/tag.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;

  let res = client.invoke('purser.domain.v1.TagService/CreateTag', {
    tag: { id: id, key: 'genre', value: 'gonzo', scope: 'TAG_SCOPE_METADATA' },
  });
  check(res, {
    'CreateTag status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateTag returns the id': (r) => r && r.message && r.message.tag && r.message.tag.id === id,
  });

  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: id });
  check(res, {
    'GetTag status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetTag returns the created value': (r) => r && r.message && r.message.tag && r.message.tag.value === 'gonzo',
  });

  res = client.invoke('purser.domain.v1.TagService/UpdateTag', {
    tag: { id: id, value: 'action' },
    updateMask: 'value',
  });
  check(res, {
    'UpdateTag status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateTag applied the field-masked value': (r) => r && r.message && r.message.tag && r.message.tag.value === 'action',
  });

  res = client.invoke('purser.domain.v1.TagService/ListTags', { pageSize: 10 });
  check(res, {
    'ListTags status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListTags includes the created tag': (r) => r && r.message && r.message.tags && r.message.tags.some((t) => t.id === id),
  });

  res = client.invoke('purser.domain.v1.TagService/DeleteTag', { id: id });
  check(res, { 'DeleteTag status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.TagService/GetTag', { id: id });
  check(res, { 'GetTag after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
