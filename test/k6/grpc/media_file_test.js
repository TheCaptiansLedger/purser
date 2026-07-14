// k6 gRPC suite for MediaFileService. See test/k6/grpc/person_test.js for
// the pattern this follows.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/media_file.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  // id is server-generated (docs/adr/0020-server-generated-kernel-entity-ids.md)
  // — never sent on Create, always read back from the response.
  let res = invoke('purser.domain.v1.MediaFileService/CreateMediaFile', {
    mediaFile: { itemId: 'item1', path: '/media/k6.mkv' },
  });
  check(res, {
    'CreateMediaFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateMediaFile returns an id': (r) => r && r.message && r.message.mediaFile && !!r.message.mediaFile.id,
  });
  const id = res.message.mediaFile.id;

  res = invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: id });
  check(res, {
    'GetMediaFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetMediaFile returns the created path': (r) => r && r.message && r.message.mediaFile && r.message.mediaFile.path === '/media/k6.mkv',
  });

  res = invoke('purser.domain.v1.MediaFileService/UpdateMediaFile', {
    mediaFile: { id: id, path: '/media/k6-updated.mkv' },
    updateMask: 'path',
  });
  check(res, {
    'UpdateMediaFile status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateMediaFile applied the field-masked path': (r) =>
      r && r.message && r.message.mediaFile && r.message.mediaFile.path === '/media/k6-updated.mkv',
  });

  res = invoke('purser.domain.v1.MediaFileService/ListMediaFiles', { pageSize: 10 });
  check(res, {
    'ListMediaFiles status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListMediaFiles includes the created file': (r) => r && r.message && r.message.mediaFiles && r.message.mediaFiles.some((m) => m.id === id),
  });

  res = invoke('purser.domain.v1.MediaFileService/ListMediaFiles', { itemId: 'item1', pageSize: 10 });
  check(res, {
    'ListMediaFiles filtered by itemId includes the created file': (r) =>
      r && r.message && r.message.mediaFiles && r.message.mediaFiles.some((m) => m.id === id),
  });

  res = invoke('purser.domain.v1.MediaFileService/ListMediaFiles', { itemId: 'no-such-item', pageSize: 10 });
  check(res, {
    'ListMediaFiles filtered by a non-matching itemId excludes the created file': (r) =>
      r && r.message && !(r.message.mediaFiles || []).some((m) => m.id === id),
  });

  res = invoke('purser.domain.v1.MediaFileService/DeleteMediaFile', { id: id });
  check(res, { 'DeleteMediaFile status is OK': (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.domain.v1.MediaFileService/GetMediaFile', { id: id });
  check(res, { 'GetMediaFile after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
