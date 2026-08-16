import { useMutation } from '@connectrpc/connect-query'
import { flushCache } from '../gen/purser/cache/v1/cache-CacheService_connectquery'

// useFlushCacheMutation wraps CacheService.FlushCache (see
// proto/purser/cache/v1/cache.proto) — the Cache tab's (#617) per-card
// flush button. Kept separate from useCacheStats, same one-hook-per-RPC
// shape useSettings.ts established for its mutations. Doesn't
// auto-invalidate useCacheStats' query cache; CacheTab calls the query's
// own refetch() after a successful flush instead, for the same reason
// ConfigTab does for updateSettings/resetSetting.
export function useFlushCacheMutation() {
  return useMutation(flushCache)
}
