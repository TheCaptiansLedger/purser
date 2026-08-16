// k6 HTTP/JSON suite for CacheService. See test/k6/http/settings_test.js
// for the pattern this follows — like Settings, Cache has no Create/
// Delete/List-with-filter shape (ListCacheStats/FlushCache instead). See
// test/k6/grpc/cache_test.js's header comment for why "musicbrainz" is the
// only cache name this suite assumes exists.
import http from 'k6/http';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const BASE_URL = __ENV.PURSER_HTTP_URL || 'http://localhost:7474';
const SERVICE = `${BASE_URL}/purser.cache.v1.CacheService`;
const HEADERS = { headers: { 'Content-Type': 'application/json' } };

function invoke(url, body, headers) {
  const res = http.post(url, body, headers);
  console.log(JSON.stringify({ method: url, request: JSON.parse(body), response: res.json() }, null, 2));
  return res;
}

export default () => {
  let res = invoke(`${SERVICE}/ListCacheStats`, JSON.stringify({}), HEADERS);
  check(res, {
    'ListCacheStats status is 200': (r) => r.status === 200,
    'ListCacheStats returns caches': (r) => Array.isArray(r.json('caches')),
  });

  // See test/k6/grpc/cache_test.js's comment: a fresh cache's all-zero
  // Stats are omitted entirely by protojson, so only presence is checked
  // here — the numeric fields round-trip is proven by
  // internal/service/cache_test.go against a populated fake cache.
  const musicbrainz = (res.json('caches') || []).find((c) => c.name === 'musicbrainz');
  check(musicbrainz, { 'musicbrainz cache is registered': (c) => !!c });

  // FlushCache on a single registered cache.
  res = invoke(`${SERVICE}/FlushCache`, JSON.stringify({ name: 'musicbrainz' }), HEADERS);
  check(res, {
    'FlushCache status is 200': (r) => r.status === 200,
    'FlushCache reports the flushed cache': (r) => {
      const flushed = r.json('flushed');
      return Array.isArray(flushed) && flushed.length === 1 && flushed[0] === 'musicbrainz';
    },
  });

  // FlushCache with an empty name flushes every registered cache.
  res = invoke(`${SERVICE}/FlushCache`, JSON.stringify({}), HEADERS);
  check(res, {
    'FlushCache(all) status is 200': (r) => r.status === 200,
    'FlushCache(all) reports musicbrainz among the flushed caches': (r) => (r.json('flushed') || []).includes('musicbrainz'),
  });

  // FlushCache on an unknown name is rejected.
  res = invoke(`${SERVICE}/FlushCache`, JSON.stringify({ name: 'no-such-cache' }), HEADERS);
  check(res, { 'FlushCache on an unknown name is 404 (NotFound)': (r) => r.status === 404 });
};
