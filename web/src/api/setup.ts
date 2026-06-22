import { useMutation, useQuery } from '@tanstack/react-query'
import { get, postEmpty } from './client'

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

export function useCompleteSetup() {
  return useMutation({
    mutationFn: () => postEmpty('/setup/complete'),
  })
}
