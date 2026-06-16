import { useQuery } from '@tanstack/react-query'
import { getPage } from './client'
import type { Job } from '../types'

export const JOBS_DISPLAY_LIMIT = 5

export function useJobs() {
  return useQuery({
    queryKey: ['jobs'],
    queryFn: () => getPage<Job>('/jobs'),
    staleTime: 0,
    refetchInterval: 2000,
  })
}

export async function cancelJob(id: string): Promise<void> {
  const res = await fetch(`/api/v1/jobs/${id}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
}
