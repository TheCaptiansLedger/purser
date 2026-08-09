// k6 flow (HTTP/JSON twin of acquisition_search_and_submit_test.js):
// "search an indexer, submit the pick, watch it, remove it" — see that
// script's header for the fixture-data rationale this mirrors over
// Connect's HTTP/JSON transport.
import http from 'k6/http';
import { check, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const INDEXER_SERVICE = `${BASE_URL}/purser.acquisition.v1.IndexerService`;
const DOWNLOAD_SERVICE = `${BASE_URL}/purser.acquisition.v1.DownloadService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

const KNOWN_QUERY = 'K6 Fixture Release';
const KNOWN_GUID = 'k6-fixture-release-guid';

function invoke(url, body, label) {
  const res = http.post(url, JSON.stringify(body), HEADERS);
  const ok = check(res, { [`${label} status is 200`]: (r) => r.status === 200 });
  if (!ok) {
    fail(`${label} failed: ${res.status} ${res.body}`);
  }
  console.log(JSON.stringify({ method: url, request: body, response: res.json() }, null, 2));
  return res;
}

export default () => {
  const searchRes = invoke(`${INDEXER_SERVICE}/Search`, { query: KNOWN_QUERY }, 'Search');
  const release = (searchRes.json('releases') || []).find((rel) => rel.guid === KNOWN_GUID);
  if (!release) {
    fail(`acquisition_search_and_submit: fixture release ${KNOWN_GUID} not found in Search results`);
  }

  const submitted = invoke(
    `${DOWNLOAD_SERVICE}/SubmitDownload`,
    { protocol: release.protocol, downloadUrl: release.downloadUrl, title: release.title, category: 'k6-fixture' },
    'SubmitDownload'
  );
  check(submitted, { 'SubmitDownload returns an external id': (r) => !!r.json('externalId') });
  const externalId = submitted.json('externalId');

  const status = invoke(`${DOWNLOAD_SERVICE}/GetDownloadStatus`, { protocol: release.protocol, externalId: externalId }, 'GetDownloadStatus (submitted)');
  check(status, {
    'GetDownloadStatus reports the submitted external id': (r) => r.json('status') && r.json('status').externalId === externalId,
    'GetDownloadStatus reports a non-unspecified state': (r) => r.json('status') && r.json('status').state !== 'DOWNLOAD_STATE_UNSPECIFIED',
  });

  invoke(`${DOWNLOAD_SERVICE}/RemoveDownload`, { protocol: release.protocol, externalId: externalId, deleteFiles: false }, 'RemoveDownload');

  const afterRemove = http.post(`${DOWNLOAD_SERVICE}/GetDownloadStatus`, JSON.stringify({ protocol: release.protocol, externalId: externalId }), HEADERS);
  console.log(JSON.stringify({ method: `${DOWNLOAD_SERVICE}/GetDownloadStatus`, response: afterRemove.body }, null, 2));
  check(afterRemove, { 'GetDownloadStatus after Remove is 404': (r) => r.status === 404 });
};
