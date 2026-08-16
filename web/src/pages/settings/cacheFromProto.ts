import type { ListCacheStatsResponse } from '../../gen/purser/cache/v1/cache_pb'
import type { CacheStats } from '../../types'

// cacheStatsFromProto converts one wire purser.cache.v1.ListCacheStatsResponse
// into the plain CacheStats[] app code is written against
// (web/src/types/index.ts) — mirrors databaseInfoFromProto's shape. Order
// is preserved as-is: ListCacheStats already returns caches sorted by
// name (internal/ports.CacheRegistry.Names), so there's nothing left for
// the frontend to sort.
export function cacheStatsFromProto(response: ListCacheStatsResponse): CacheStats[] {
  return response.caches.map(cache => ({
    name: cache.name,
    items: cache.items,
    bytes: Number(cache.bytes),
    hits: Number(cache.hits),
    misses: Number(cache.misses),
  }))
}
