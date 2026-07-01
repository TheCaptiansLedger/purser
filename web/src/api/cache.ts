import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { del, get } from './client'

export interface CacheStats {
  name:   string
  hits:   number
  misses: number
  size:   number
  bytes:  number
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

export function useFlushCache() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => del(`/cache/${encodeURIComponent(name)}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['cache', 'stats'] }),
  })
}
