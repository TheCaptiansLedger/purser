import { useQuery } from '@tanstack/react-query'
import { get } from './client'

export interface CacheStats {
  name:   string
  hits:   number
  misses: number
  size:   number
}

interface CacheStatsResponse {
  caches: CacheStats[]
}

export function useCacheStats() {
  return useQuery({
    queryKey: ['cache', 'stats'],
    queryFn: () => get<CacheStatsResponse>('/cache/stats'),
    refetchInterval: 2000,
    staleTime: 0,
  })
}
