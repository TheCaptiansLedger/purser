// k6 gRPC suite for ImageService. See test/k6/grpc/person_test.js for the
// pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:8080';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/image.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;
  const ownerId = 'k6-owner-1';

  let res = client.invoke('purser.domain.v1.ImageService/CreateImage', {
    image: { id: id, ownerType: 'person', ownerId: ownerId, imageType: 'poster', url: 'https://example.com/k6.jpg' },
  });
  check(res, {
    'CreateImage status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateImage returns the id': (r) => r && r.message && r.message.image && r.message.image.id === id,
  });

  res = client.invoke('purser.domain.v1.ImageService/GetImage', { id: id });
  check(res, {
    'GetImage status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetImage returns the created url': (r) => r && r.message && r.message.image && r.message.image.url === 'https://example.com/k6.jpg',
  });

  res = client.invoke('purser.domain.v1.ImageService/UpdateImage', {
    image: { id: id, url: 'https://example.com/k6-updated.jpg' },
    updateMask: 'url',
  });
  check(res, {
    'UpdateImage status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateImage applied the field-masked url': (r) => r && r.message && r.message.image && r.message.image.url === 'https://example.com/k6-updated.jpg',
  });

  res = client.invoke('purser.domain.v1.ImageService/ListImages', { ownerType: 'person', ownerId: ownerId, pageSize: 10 });
  check(res, {
    'ListImages status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListImages includes the created image': (r) => r && r.message && r.message.images && r.message.images.some((img) => img.id === id),
  });

  res = client.invoke('purser.domain.v1.ImageService/DeleteImage', { id: id });
  check(res, { 'DeleteImage status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ImageService/GetImage', { id: id });
  check(res, { 'GetImage after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
