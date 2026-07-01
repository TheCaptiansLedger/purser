import { useCacheStats } from '../../api/cache'
import type { CacheStats } from '../../api/cache'

function hitRate(s: CacheStats): number {
  const total = s.hits + s.misses
  return total === 0 ? 0 : Math.round((s.hits / total) * 100)
}

function rateColor(pct: number): string {
  if (pct >= 80) return '#10b981'
  if (pct >= 50) return '#f59e0b'
  return '#ef4444'
}

function CacheCard({ stat }: { stat: CacheStats }) {
  const pct = hitRate(stat)
  const color = rateColor(pct)
  const total = stat.hits + stat.misses

  return (
    <div className="rounded-xl border border-white/5 bg-white/2 px-5 py-4 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-semibold text-white/85 font-mono">{stat.name}</span>
        <span
          className="text-xs font-bold px-2 py-0.5 rounded-full font-mono"
          style={{ background: `${color}18`, color }}
        >
          {pct}% hit rate
        </span>
      </div>

      <div className="w-full h-1.5 rounded-full bg-white/5 overflow-hidden">
        <div
          className="h-full rounded-full transition-all duration-500"
          style={{ width: `${pct}%`, background: color }}
        />
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div className="flex flex-col gap-0.5">
          <span className="text-[10px] font-semibold uppercase tracking-widest text-white/25">Hits</span>
          <span className="text-sm font-mono text-emerald-400">{stat.hits.toLocaleString()}</span>
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-[10px] font-semibold uppercase tracking-widest text-white/25">Misses</span>
          <span className="text-sm font-mono text-white/50">{stat.misses.toLocaleString()}</span>
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-[10px] font-semibold uppercase tracking-widest text-white/25">Size</span>
          <span className="text-sm font-mono text-white/50">{stat.size.toLocaleString()}</span>
        </div>
      </div>

      {total > 0 && (
        <p className="text-[11px] text-white/25 font-mono">
          {total.toLocaleString()} total requests
        </p>
      )}
    </div>
  )
}

function SkeletonCard() {
  return (
    <div className="rounded-xl border border-white/5 bg-white/2 px-5 py-4 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div className="h-4 w-24 bg-white/5 rounded animate-pulse" />
        <div className="h-5 w-20 bg-white/5 rounded-full animate-pulse" />
      </div>
      <div className="h-1.5 w-full bg-white/5 rounded-full animate-pulse" />
      <div className="grid grid-cols-3 gap-3">
        {[0, 1, 2].map(i => (
          <div key={i} className="flex flex-col gap-1">
            <div className="h-2.5 w-10 bg-white/5 rounded animate-pulse" />
            <div className="h-4 w-14 bg-white/5 rounded animate-pulse" />
          </div>
        ))}
      </div>
    </div>
  )
}

export function CacheStatsPage() {
  const { data, isLoading, error } = useCacheStats()

  return (
    <div className="px-8 py-10 max-w-2xl">
      <div className="mb-8 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-white">Cache</h1>
          <p className="text-white/40 text-sm mt-1">Hit/miss diagnostics — refreshes every 2 seconds</p>
        </div>
        <span className="flex items-center gap-1.5 text-[11px] text-white/25 mt-1.5">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
          live
        </span>
      </div>

      {error && (
        <div className="rounded-xl border border-red-500/20 bg-red-500/8 px-4 py-3 text-red-400 text-sm mb-6">
          {error.message}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {isLoading
          ? [0, 1].map(i => <SkeletonCard key={i} />)
          : (data?.caches ?? []).map(stat => (
              <CacheCard key={stat.name} stat={stat} />
            ))
        }
        {!isLoading && (data?.caches ?? []).length === 0 && (
          <div className="rounded-xl border border-white/5 bg-white/1 py-10 text-center text-sm text-white/25">
            No caches registered.
          </div>
        )}
      </div>
    </div>
  )
}
