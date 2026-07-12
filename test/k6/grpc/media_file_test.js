// k6 gRPC suite for MediaFileService. See test/k6/grpc/person_test.js for
// the pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:8080';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/media_file.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const id = `k6-grpc-${__VU}-${__ITER}-${Date.now()}`;

  let res = client.invoke('purser.domain.v1.MediaFileService/CreateMediaFile', {
    mediaFile: { id: id, itemId: 'item1', path: '/media/k6.mkv' },
  });
  check(res, {
    'CreateMediaFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateMediaFile returns the id': (r) => r && r.message && r.message.mediaFile && r.message.mediaFile.id === id,
  });

  res = client.invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: id });
  check(res, {
    'GetMediaFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetMediaFile returns the created path': (r) => r && r.message && r.message.mediaFile && r.message.mediaFile.path === '/media/k6.mkv',
  });

  res = client.invoke('purser.domain.v1.MediaFileService/UpdateMediaFile', {
    mediaFile: { id: id, path: '/media/k6-updated.mkv' },
    updateMask: 'path',
  });
  check(res, {
    'UpdateMediaFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateMediaFile applied the field-masked path': (r) =>
      r && r.message && r.message.mediaFile && r.message.mediaFile.path === '/media/k6-updated.mkv',
  });

  res = client.invoke('purser.domain.v1.MediaFileService/ListMediaFiles', { pageSize: 10 });
  check(res, {
    'ListMediaFiles status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListMediaFiles includes the created file': (r) => r && r.message && r.message.mediaFiles && r.message.mediaFiles.some((m) => m.id === id),
  });

  res = client.invoke('purser.domain.v1.MediaFileService/DeleteMediaFile', { id: id });
  check(res, { 'DeleteMediaFile status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: id });
  check(res, { 'GetMediaFile after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
