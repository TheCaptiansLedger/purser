// k6 HTTP/JSON suite for DownloadService — see
// test/k6/grpc/download_test.js for the fixture-data rationale this
// mirrors over Connect's HTTP/JSON transport.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.acquisition.v1.DownloadService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

function exerciseProtocol(protocol, downloadUrl) {
  let res = invoke(
    `${SERVICE}/SubmitDownload`,
    JSON.stringify({ protocol: protocol, downloadUrl: downloadUrl, title: 'K6 Fixture Download', category: 'k6-fixture' }),
    HEADERS
  );
  check(res, {
    [`SubmitDownload (${protocol}) status is 200`]: (r) => r.status === 200,
    [`SubmitDownload (${protocol}) returns an external id`]: (r) => !!r.json('externalId'),
  });
  const externalId = res.json('externalId');

  res = invoke(`${SERVICE}/GetDownloadStatus`, JSON.stringify({ protocol: protocol, externalId: externalId }), HEADERS);
  check(res, {
    [`GetDownloadStatus (${protocol}) status is 200`]: (r) => r.status === 200,
    [`GetDownloadStatus (${protocol}) reports the submitted external id`]: (r) => r.json('status') && r.json('status').externalId === externalId,
    [`GetDownloadStatus (${protocol}) reports a non-unspecified state`]: (r) => r.json('status') && r.json('status').state !== 'DOWNLOAD_STATE_UNSPECIFIED',
  });

  res = invoke(`${SERVICE}/RemoveDownload`, JSON.stringify({ protocol: protocol, externalId: externalId, deleteFiles: false }), HEADERS);
  check(res, { [`RemoveDownload (${protocol}) status is 200`]: (r) => r.status === 200 });

  res = invoke(`${SERVICE}/GetDownloadStatus`, JSON.stringify({ protocol: protocol, externalId: externalId }), HEADERS);
  check(res, { [`GetDownloadStatus (${protocol}) after Remove is 404`]: (r) => r.status === 404 });
}

export default () => {
  exerciseProtocol('PROTOCOL_TORRENT', 'magnet:?xt=urn:btih:k6-fixture-http');
  exerciseProtocol('PROTOCOL_USENET', 'http://sabnzbd.invalid/k6-fixture-http.nzb');

  const res = invoke(`${SERVICE}/SubmitDownload`, JSON.stringify({ protocol: 'PROTOCOL_UNSPECIFIED', downloadUrl: 'magnet:?xt=urn:btih:k6-fixture-http-unsupported' }), HEADERS);
  check(res, { 'SubmitDownload with an unsupported protocol is 400': (r) => r.status === 400 });
};
