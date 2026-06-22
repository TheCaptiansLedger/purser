import { useQuery } from '@tanstack/react-query'
import { get } from './client'

export interface SetupStatus {
  complete: boolean
}

export function useSetupStatus() {
  return useQuery({
    queryKey: ['setup', 'status'],
    queryFn: () => get<SetupStatus>('/setup/status'),
    staleTime: Infinity,
  })
}
