// k6 gRPC suite for IndexerService — the read-only search half of the
// acquisition pipeline (#579/#584; submission is #585). Exercises the
// real Prowlarr-shaped decode path against
// internal/adapters/prowlarr/fixtureserver's canned data (Make's
// _k6-app-start sets PURSER_PROWLARR_MOCK=1 and a placeholder
// prowlarr: config block — see cmd/purser/serve.go's newIndexerSearcher),
// no live network required. KNOWN_QUERY/KNOWN_GUID/KNOWN_TITLE mirror
// fixtureserver's own constants — Go and JS can't share constants
// directly, keep any change to either in sync.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const KNOWN_QUERY = 'K6 Fixture Release';
const KNOWN_GUID = 'k6-fixture-release-guid';
const KNOWN_TITLE = 'K6 Fixture Release 1080p';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/acquisition/v1/indexer.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  let res = invoke('purser.acquisition.v1.IndexerService/Search', { query: KNOWN_QUERY });
  check(res, {
    'Search status is OK': (r) => r && r.status === grpc.StatusOK,
    'Search returns the fixture release': (r) =>
      r && r.message && r.message.releases && r.message.releases.some((rel) => rel.guid === KNOWN_GUID && rel.title === KNOWN_TITLE),
  });
  const release = res.message.releases.find((rel) => rel.guid === KNOWN_GUID);
  check(release, {
    'fixture release reports protocol torrent': (rel) => rel.protocol === 'PROTOCOL_TORRENT',
    'fixture release reports seeders/leechers': (rel) => rel.seeders === 42 && rel.leechers === 3,
    'fixture release reports its category': (rel) => rel.categories && rel.categories.some((c) => c.id === 3000 && c.name === 'Music'),
  });

  res = invoke('purser.acquisition.v1.IndexerService/Search', { query: 'no-such-release-k6-grpc' });
  check(res, {
    'Search for an unmatched query is OK': (r) => r && r.status === grpc.StatusOK,
    'Search for an unmatched query returns an empty, non-error result': (r) => r && r.message && (r.message.releases || []).length === 0,
  });

  client.close();
};
