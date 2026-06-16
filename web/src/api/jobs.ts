import { useQuery } from '@tanstack/react-query'
import { getPage } from './client'
import type { Job } from '../types'

export function useJobs() {
  return useQuery({
    queryKey: ['jobs'],
    queryFn: () => getPage<Job>('/jobs'),
    refetchInterval: (query) => {
      const jobs = query.state.data?.data ?? []
      return jobs.some(j => j.status === 'queued' || j.status === 'running') ? 2000 : false
    },
  })
}

export async function cancelJob(id: string): Promise<void> {
  const res = await fetch(`/api/v1/jobs/${id}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
}
