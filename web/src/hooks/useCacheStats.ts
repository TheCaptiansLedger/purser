import { useQuery } from '@connectrpc/connect-query'
import { listCacheStats } from '../gen/purser/cache/v1/cache-CacheService_connectquery'

// CACHE_STATS_POLL_INTERVAL_MS matches the pre-reset CacheStatsPage's own
// 2-second refresh (see #617) — cache hit/miss counters change on every
// request elsewhere in the app, so a one-shot query would go stale the
// moment the tab is opened.
const CACHE_STATS_POLL_INTERVAL_MS = 2_000

// useCacheStats wraps CacheService.ListCacheStats (see
// proto/purser/cache/v1/cache.proto) — the Cache tab's (#617) live read of
// every registered cache's hit/miss/entry/memory stats.
export function useCacheStats() {
  return useQuery(listCacheStats, {}, { refetchInterval: CACHE_STATS_POLL_INTERVAL_MS })
}
