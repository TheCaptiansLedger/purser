// k6 HTTP/JSON suite for IndexerService — see test/k6/grpc/indexer_test.js
// for the fixture-data rationale this mirrors over Connect's HTTP/JSON
// transport.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.acquisition.v1.IndexerService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

const KNOWN_QUERY = 'K6 Fixture Release';
const KNOWN_GUID = 'k6-fixture-release-guid';
const KNOWN_TITLE = 'K6 Fixture Release 1080p';

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  let res = invoke(`${SERVICE}/Search`, JSON.stringify({ query: KNOWN_QUERY }), HEADERS);
  check(res, {
    'Search status is 200': (r) => r.status === 200,
    'Search returns the fixture release': (r) =>
      (r.json('releases') || []).some((rel) => rel.guid === KNOWN_GUID && rel.title === KNOWN_TITLE),
  });
  const release = res.json('releases').find((rel) => rel.guid === KNOWN_GUID);
  check(release, {
    'fixture release reports protocol torrent': (rel) => rel.protocol === 'PROTOCOL_TORRENT',
    'fixture release reports seeders/leechers': (rel) => rel.seeders === 42 && rel.leechers === 3,
    'fixture release reports its category': (rel) => (rel.categories || []).some((c) => c.id === 3000 && c.name === 'Music'),
  });

  res = invoke(`${SERVICE}/Search`, JSON.stringify({ query: 'no-such-release-k6-http' }), HEADERS);
  check(res, {
    'Search for an unmatched query is 200': (r) => r.status === 200,
    'Search for an unmatched query returns an empty, non-error result': (r) => (r.json('releases') || []).length === 0,
  });
};
