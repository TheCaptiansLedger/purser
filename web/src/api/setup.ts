import { useMutation, useQuery } from '@tanstack/react-query'
import { get, post, postEmpty } from './client'

export interface SetupStatus {
  complete: boolean
}

export interface VerifySourceResponse {
  ok: boolean
  error?: string
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

export function useVerifySource() {
  return useMutation({
    mutationFn: (source: string) => post<VerifySourceResponse>('/verify/source', { source }),
  })
}
