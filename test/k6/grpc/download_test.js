// k6 gRPC suite for DownloadService — the submission half of the
// acquisition pipeline (#579/#585; search is #584). Exercises the real
// qBittorrent/SABnzbd-shaped request/response paths against
// internal/adapters/qbittorrent/fixtureserver's and
// internal/adapters/sabnzbd/fixtureserver's canned data (Make's
// _k6-app-start sets PURSER_QBITTORRENT_MOCK=1/PURSER_SABNZBD_MOCK=1 and
// placeholder qbittorrent:/sabnzbd: config blocks — see
// cmd/purser/serve.go's newQBittorrentClient/newSABnzbdClient), no live
// network required. Full lifecycle per protocol: submit, check status,
// remove, check status again (not found) — see
// docs/adr/0011-api-design.md's k6 testing section for that convention.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/acquisition/v1/download.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

function exerciseProtocol(protocol, downloadUrl) {
  let res = invoke('purser.acquisition.v1.DownloadService/SubmitDownload', {
    protocol: protocol,
    download_url: downloadUrl,
    title: 'K6 Fixture Download',
    category: 'k6-fixture',
  });
  check(res, {
    [`SubmitDownload (${protocol}) status is OK`]: (r) => r && r.status === grpc.StatusOK,
    [`SubmitDownload (${protocol}) returns an external id`]: (r) => r && r.message && r.message.externalId,
  });
  const externalId = res.message.externalId;

  res = invoke('purser.acquisition.v1.DownloadService/GetDownloadStatus', { protocol: protocol, external_id: externalId });
  check(res, {
    [`GetDownloadStatus (${protocol}) status is OK`]: (r) => r && r.status === grpc.StatusOK,
    [`GetDownloadStatus (${protocol}) reports the submitted external id`]: (r) => r && r.message && r.message.status && r.message.status.externalId === externalId,
    [`GetDownloadStatus (${protocol}) reports a non-unspecified state`]: (r) => r && r.message && r.message.status && r.message.status.state !== 'DOWNLOAD_STATE_UNSPECIFIED',
  });

  res = invoke('purser.acquisition.v1.DownloadService/RemoveDownload', { protocol: protocol, external_id: externalId, delete_files: false });
  check(res, { [`RemoveDownload (${protocol}) status is OK`]: (r) => r && r.status === grpc.StatusOK });

  res = invoke('purser.acquisition.v1.DownloadService/GetDownloadStatus', { protocol: protocol, external_id: externalId });
  check(res, { [`GetDownloadStatus (${protocol}) after Remove is NOT_FOUND`]: (r) => r && r.status === grpc.StatusNotFound });
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  exerciseProtocol('PROTOCOL_TORRENT', 'magnet:?xt=urn:btih:k6-fixture-grpc');
  exerciseProtocol('PROTOCOL_USENET', 'http://sabnzbd.invalid/k6-fixture-grpc.nzb');

  const res = invoke('purser.acquisition.v1.DownloadService/SubmitDownload', {
    protocol: 'PROTOCOL_UNSPECIFIED',
    download_url: 'magnet:?xt=urn:btih:k6-fixture-grpc-unsupported',
  });
  check(res, { 'SubmitDownload with an unsupported protocol is INVALID_ARGUMENT': (r) => r && r.status === grpc.StatusInvalidArgument });

  client.close();
};
