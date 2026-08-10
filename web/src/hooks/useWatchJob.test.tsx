import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { JobEventKind, JobService, JobStatus } from '../gen/purser/job/v1/job_pb'
import { useWatchJob } from './useWatchJob'

// ADR 0004: hooks are unit tested in isolation, against a mocked transport
// (createRouterTransport, Connect's own in-memory Transport), never a live
// backend — same pattern as useJobs.test.tsx/useJobsList.test.tsx. No
// QueryClientProvider is needed here: unlike those, this hook doesn't go
// through connect-query, so it only needs TransportProvider's context.
function wrapper(mockTransport: ReturnType<typeof createRouterTransport>) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <TransportProvider transport={mockTransport}>{children}</TransportProvider>
  }
}

describe('useWatchJob', () => {
  it('returns undefined state and opens no stream when jobId is undefined', () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        // eslint-disable-next-line require-yield
        watchJob: async function* () {
          throw new Error('watchJob should not be called without a jobId')
        },
      })
    })

    const { result } = renderHook(() => useWatchJob(undefined), { wrapper: wrapper(mockTransport) })

    expect(result.current).toEqual({ job: undefined, event: undefined, error: undefined, done: false })
  })

  it('applies each JobEvent snapshot as it streams in, then marks done when the stream closes', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        watchJob: async function* () {
          yield {
            kind: JobEventKind.JOB,
            job: { id: 'job-1', status: JobStatus.RUNNING, tasks: [] },
          }
          yield {
            kind: JobEventKind.TASK,
            taskId: 'task-1',
            job: { id: 'job-1', status: JobStatus.SUCCEEDED, tasks: [{ id: 'task-1', label: 'track one' }] },
          }
        },
      })
    })

    const { result } = renderHook(() => useWatchJob('job-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.done).toBe(true))
    expect(result.current.job?.status).toBe(JobStatus.SUCCEEDED)
    expect(result.current.job?.tasks).toHaveLength(1)
    expect(result.current.event?.kind).toBe(JobEventKind.TASK)
    expect(result.current.error).toBeUndefined()
  })

  it('surfaces a ConnectError when the stream fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        // eslint-disable-next-line require-yield
        watchJob: async function* () {
          throw new Error('unavailable')
        },
      })
    })

    const { result } = renderHook(() => useWatchJob('job-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.error).toBeTruthy())
    expect(result.current.done).toBe(false)
  })

  it('tears down the previous stream and resets state when jobId changes', async () => {
    let job2Started = false
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        watchJob: async function* (req) {
          if (req.jobId === 'job-1') {
            // Never resolves within the test's lifetime — proves that
            // switching jobId doesn't leave this stream's events landing
            // on the new state.
            yield { kind: JobEventKind.JOB, job: { id: 'job-1', status: JobStatus.RUNNING, tasks: [] } }
            await new Promise(() => {})
            return
          }
          job2Started = true
          yield { kind: JobEventKind.JOB, job: { id: 'job-2', status: JobStatus.SUCCEEDED, tasks: [] } }
        },
      })
    })

    const { result, rerender } = renderHook(({ jobId }) => useWatchJob(jobId), {
      wrapper: wrapper(mockTransport),
      initialProps: { jobId: 'job-1' },
    })

    await waitFor(() => expect(result.current.job?.id).toBe('job-1'))

    rerender({ jobId: 'job-2' })

    // Reset happens synchronously on the jobId change, before the new
    // stream's first event arrives.
    expect(result.current.job).toBeUndefined()

    await waitFor(() => expect(result.current.job?.id).toBe('job-2'))
    expect(job2Started).toBe(true)
    expect(result.current.job?.status).toBe(JobStatus.SUCCEEDED)
  })
})
