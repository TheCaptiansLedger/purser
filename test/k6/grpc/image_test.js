// k6 gRPC suite for ImageService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/image.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // id is server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  const ownerId = 'k6-owner-1';

  let res = invoke('purser.domain.v1.ImageService/CreateImage', {
    image: { ownerType: 'person', ownerId: ownerId, imageType: 'poster', url: 'https://example.com/k6.jpg' },
  });
  check(res, {
    'CreateImage status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateImage returns an id': (r) => r && r.message && r.message.image && !!r.message.image.id,
  });
  const id = res.message.image.id;

  res = invoke('purser.domain.v1.ImageService/GetImage', { id: id });
  check(res, {
    'GetImage status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetImage returns the created url': (r) => r && r.message && r.message.image && r.message.image.url === 'https://example.com/k6.jpg',
  });

  res = invoke('purser.domain.v1.ImageService/UpdateImage', {
    image: { id: id, url: 'https://example.com/k6-updated.jpg' },
    updateMask: 'url',
  });
  check(res, {
    'UpdateImage status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateImage applied the field-masked url': (r) => r && r.message && r.message.image && r.message.image.url === 'https://example.com/k6-updated.jpg',
  });

  res = invoke('purser.domain.v1.ImageService/ListImages', { ownerType: 'person', ownerId: ownerId, pageSize: 10 });
  check(res, {
    'ListImages status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListImages includes the created image': (r) => r && r.message && r.message.images && r.message.images.some((img) => img.id === id),
  });

  res = invoke('purser.domain.v1.ImageService/DeleteImage', { id: id });
  check(res, { 'DeleteImage status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.ImageService/GetImage', { id: id });
  check(res, { 'GetImage after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
