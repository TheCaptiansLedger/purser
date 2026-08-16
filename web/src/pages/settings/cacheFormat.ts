// hitRate renders a cache's cumulative hit rate as a whole-number
// percentage, the way the Cache tab's (#617) per-cache cards need it.
// Matches the pre-reset CacheStatsPage's own rounding convention (prior
// art for this tab's UX, not resurrected code). A cache with no requests
// yet (hits + misses === 0) reads as 0%, not NaN.
export function hitRate(hits: number, misses: number): number {
  const total = hits + misses
  return total === 0 ? 0 : Math.round((hits / total) * 100)
}
