// k6 flow: "search an indexer, submit the pick, watch it, remove it" — the
// task a UI/admin tool runs to acquire something Purser doesn't have yet
// (#579/#586). Chains IndexerService.Search (#584) into DownloadService's
// Submit/GetStatus/Remove (#585), exactly the two-call sequence
// docs/technical/acquisition-pipeline.md's "Data flow" section describes:
// a caller searches, picks a release, and hands the fields it needs off
// that release (protocol, download_url, title) to SubmitDownload — the two
// services never call each other directly, per 0011's no-God-service rule.
//
// Runs against internal/adapters/prowlarr/fixtureserver's canned search
// result and internal/adapters/qbittorrent/fixtureserver's canned torrent
// lifecycle (Make's _k6-app-start sets PURSER_PROWLARR_MOCK=1 and
// PURSER_QBITTORRENT_MOCK=1 — see cmd/purser/serve.go's
// newIndexerSearcher/newQBittorrentClient), no live network required. The
// fixture release always reports protocol torrent, so this flow only
// exercises the torrent path end to end; the usenet protocol already has
// its own full lifecycle coverage in test/k6/grpc/download_test.js.
// KNOWN_QUERY/KNOWN_GUID mirror prowlarr/fixtureserver's own constants —
// Go and JS can't share constants directly, keep any change to either in
// sync.
import grpc from 'k6/net/grpc';
import { check, fail } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const KNOWN_QUERY = 'K6 Fixture Release';
const KNOWN_GUID = 'k6-fixture-release-guid';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/acquisition/v1/indexer.proto', 'purser/acquisition/v1/download.proto');

function invoke(method, request, label) {
  const res = client.invoke(method, request);
  const ok = check(res, { [`${label} status is OK`]: (r) => r && r.status === grpc.StatusOK });
  if (!ok) {
    fail(`${label} failed: ${res && res.status} ${res && res.error && res.error.message}`);
  }
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res.message;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  const searchRes = invoke('purser.acquisition.v1.IndexerService/Search', { query: KNOWN_QUERY }, 'Search');
  const release = (searchRes.releases || []).find((rel) => rel.guid === KNOWN_GUID);
  if (!release) {
    fail(`acquisition_search_and_submit: fixture release ${KNOWN_GUID} not found in Search results`);
  }

  const submitted = invoke(
    'purser.acquisition.v1.DownloadService/SubmitDownload',
    {
      protocol: release.protocol,
      download_url: release.downloadUrl,
      title: release.title,
      category: 'k6-fixture',
    },
    'SubmitDownload'
  );
  check(submitted, { 'SubmitDownload returns an external id': (r) => !!r.externalId });
  const externalId = submitted.externalId;

  let status = invoke(
    'purser.acquisition.v1.DownloadService/GetDownloadStatus',
    { protocol: release.protocol, external_id: externalId },
    'GetDownloadStatus (submitted)'
  );
  check(status, {
    'GetDownloadStatus reports the submitted external id': (r) => r.status && r.status.externalId === externalId,
    'GetDownloadStatus reports a non-unspecified state': (r) => r.status && r.status.state !== 'DOWNLOAD_STATE_UNSPECIFIED',
  });

  invoke('purser.acquisition.v1.DownloadService/RemoveDownload', { protocol: release.protocol, external_id: externalId, delete_files: false }, 'RemoveDownload');

  const afterRemove = client.invoke('purser.acquisition.v1.DownloadService/GetDownloadStatus', { protocol: release.protocol, external_id: externalId });
  console.log(JSON.stringify({ method: 'purser.acquisition.v1.DownloadService/GetDownloadStatus', response: afterRemove.message }, null, 2));
  check(afterRemove, { 'GetDownloadStatus after Remove is NOT_FOUND': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
