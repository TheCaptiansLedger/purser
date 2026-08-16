// k6 gRPC suite for CacheService. See test/k6/grpc/settings_test.js for the
// pattern this follows — like Settings, Cache has no Create/Delete/List-
// with-filter shape (ListCacheStats/FlushCache instead).
//
// "musicbrainz" is the one cache guaranteed present in every k6-ci run: its
// client is always constructed, mock or not (see cmd/purser/serve.go's
// newMusicIdentificationClients and PURSER_MUSICBRAINZ_MOCK). Every other
// provider cache (stashdb, theporndb, theaudiodb, fanarttv, acoustid) only
// registers when that provider is enabled, which the generated
// purser-ci.yaml doesn't do — so this suite doesn't assume any of them
// exist, only that "musicbrainz" does.
import grpc from 'k6/net/grpc';
import { check } from 'k6';
import { options } from '../lib/options.js';
export { options };

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/cache/v1/cache.proto');

function invoke(method, request) {
  const res = client.invoke(method, request);
  console.log(JSON.stringify({ method: method, request: request, response: res.message }, null, 2));
  return res;
}

export default () => {
  client.connect(ADDR, { plaintext: true });

  let res = invoke('purser.cache.v1.CacheService/ListCacheStats', {});
  check(res, {
    'ListCacheStats status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListCacheStats returns caches': (r) => r && r.message && Array.isArray(r.message.caches),
  });

  // A fresh cache's Stats are all-zero, which proto3 JSON (both native
  // gRPC's message decoding and Connect's HTTP/JSON) omits entirely rather
  // than encoding as 0 — the same zero-value-omission behavior
  // test/k6/http/settings_test.js's own comment documents for an unset
  // secret. So the only thing to assert pre-flush is that the cache is
  // registered at all; FlushCache below (and its Go/k6 coverage of a
  // populated cache in internal/service/cache_test.go) is what proves the
  // numeric fields round-trip correctly.
  const musicbrainz = res.message.caches.find((c) => c.name === 'musicbrainz');
  check(musicbrainz, { 'musicbrainz cache is registered': (c) => !!c });

  // FlushCache on a single registered cache.
  res = invoke('purser.cache.v1.CacheService/FlushCache', { name: 'musicbrainz' });
  check(res, {
    'FlushCache status is OK': (r) => r && r.status === grpc.StatusOK,
    'FlushCache reports the flushed cache': (r) => r && r.message && r.message.flushed && r.message.flushed.length === 1 && r.message.flushed[0] === 'musicbrainz',
  });

  // FlushCache with an empty name flushes every registered cache.
  res = invoke('purser.cache.v1.CacheService/FlushCache', {});
  check(res, {
    'FlushCache(all) status is OK': (r) => r && r.status === grpc.StatusOK,
    'FlushCache(all) reports musicbrainz among the flushed caches': (r) => r && r.message && r.message.flushed && r.message.flushed.includes('musicbrainz'),
  });

  // FlushCache on an unknown name is rejected.
  res = invoke('purser.cache.v1.CacheService/FlushCache', { name: 'no-such-cache' });
  check(res, { 'FlushCache on an unknown name is NotFound': (r) => r && r.status === grpc.StatusNotFound });
};
