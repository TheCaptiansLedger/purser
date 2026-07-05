import { useQuery } from '@tanstack/react-query'
import { get, getPage } from './client'
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

export function useActiveJobForEntry(entryId: string, jobName: string): Job | null {
  const { data } = useJobs()
  return (
    (data?.data ?? []).find(
      j =>
        j.name === jobName &&
        (j.status === 'queued' || j.status === 'running') &&
        j.payload?.entry_id === entryId
    ) ?? null
  )
}

export async function cancelJob(id: string): Promise<void> {
  const res = await fetch(`/api/v1/jobs/${id}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
}

// pollForItemId polls GET /api/v1/jobs/:id with exponential backoff until the
// job completes, then returns the item_id from the job result.
export async function pollForItemId(jobId: string): Promise<string> {
  const MAX_RETRIES = 8
  let delay = 500
  for (let i = 0; i < MAX_RETRIES; i++) {
    await new Promise(resolve => setTimeout(resolve, delay))
    const job = await get<Job>(`/jobs/${jobId}`)
    if (job.status === 'completed') {
      const itemId = job.result?.item_id
      if (typeof itemId !== 'string' || itemId === '') {
        throw new Error('Import job completed but returned no item_id')
      }
      return itemId
    }
    if (job.status === 'failed' || job.status === 'cancelled') {
      throw new Error(job.error ?? `Import job ${job.status}`)
    }
    delay = Math.min(delay * 2, 10_000)
  }
  throw new Error('Import job did not complete within expected time')
}
