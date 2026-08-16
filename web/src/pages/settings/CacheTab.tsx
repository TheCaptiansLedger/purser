import { AlertCircle, Loader2, Trash2 } from 'lucide-react'
import { useCacheStats } from '../../hooks/useCacheStats'
import { useFlushCacheMutation } from '../../hooks/useFlushCacheMutation'
import { cacheStatsFromProto } from './cacheFromProto'
import { formatBytes } from './databaseFormat'
import { hitRate } from './cacheFormat'
import type { CacheStats } from '../../types'

// RATE_TONE_CLASS buckets a cache's hit rate into the same
// success/warning/failure thresholds the pre-reset CacheStatsPage used
// (prior art for this tab's UX, not resurrected code) — 80%+ reads as
// healthy, 50-79% as degraded, below that as a cache not worth its
// upkeep.
function rateToneClass(pct: number): string {
  if (pct >= 80) return 'text-status-success'
  if (pct >= 50) return 'text-status-warning'
  return 'text-status-failure'
}

// CacheTab is #617's Cache tab: one card per registered cache (hit rate,
// hits/misses, entries, memory) with a per-cache flush button, polling
// ListCacheStats every 2 seconds (useCacheStats) — full UX parity with a
// pre-reset version of this feature. Built on #615's CacheService and
// #614's Cache-port Stats/Flush extension.
export function CacheTab() {
  const statsQuery = useCacheStats()
  const flush = useFlushCacheMutation()

  function handleFlush(name: string) {
    flush.mutate({ name }, { onSuccess: () => void statsQuery.refetch() })
  }

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
  // A local Connect round trip resolves well under 400ms; a loading
  // indicator here would read as slower, not more informative.
  if (statsQuery.isPending) {
    return null
  }

  if (statsQuery.isError) {
    return (
      <p className="text-status-failure text-body" role="alert">
        Couldn't load cache stats ({statsQuery.error.message}).
      </p>
    )
  }

  const caches = cacheStatsFromProto(statsQuery.data)
  const flushingName = flush.isPending ? flush.variables?.name : undefined

  return (
    <div className="flex flex-col gap-4">
      {caches.length === 0 ? (
        <section className="bg-surface border border-border rounded-lg p-5">
          <p className="text-body text-text-secondary">No caches registered.</p>
        </section>
      ) : (
        caches.map(cache => (
          <CacheCard key={cache.name} cache={cache} flushing={flushingName === cache.name} onFlush={() => handleFlush(cache.name)} />
        ))
      )}
      {flush.isError && (
        <p className="flex items-center gap-2 text-label text-status-failure" role="alert">
          <AlertCircle size={14} />
          <span>Flush failed: {flush.error.message}</span>
        </p>
      )}
    </div>
  )
}

interface CacheCardProps {
  cache: CacheStats
  flushing: boolean
  onFlush: () => void
}

function CacheCard({ cache, flushing, onFlush }: CacheCardProps) {
  const pct = hitRate(cache.hits, cache.misses)
  const stats: { label: string; value: string }[] = [
    { label: 'Hits', value: cache.hits.toLocaleString() },
    { label: 'Misses', value: cache.misses.toLocaleString() },
    { label: 'Entries', value: cache.items.toLocaleString() },
    { label: 'Memory', value: formatBytes(cache.bytes) },
  ]

  return (
    <section className="bg-surface border border-border rounded-lg p-5 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <span className="text-body font-medium text-text font-mono">{cache.name}</span>
        <div className="flex items-center gap-3">
          <span className={`text-label font-medium ${rateToneClass(pct)}`}>{pct}% hit rate</span>
          <button
            type="button"
            onClick={onFlush}
            disabled={flushing || cache.items === 0}
            title="Flush cache"
            className="flex items-center gap-1 text-label text-text-secondary hover:text-status-failure disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {flushing ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
            <span>Flush</span>
          </button>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {stats.map(stat => (
          <div key={stat.label} className="flex flex-col gap-0.5">
            <span className="text-label text-text-secondary">{stat.label}</span>
            <span className="text-body text-text font-mono">{stat.value}</span>
          </div>
        ))}
      </div>
    </section>
  )
}
