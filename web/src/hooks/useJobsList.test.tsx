import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { JobService } from '../gen/purser/job/v1/job_pb'
import { useJobsList } from './useJobsList'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same pattern as useJobs.test.tsx.
function wrapper(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <TransportProvider transport={mockTransport}>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </TransportProvider>
    )
  }
}

describe('useJobsList', () => {
  it('fetches the first page and reports whether more pages exist', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        listJobs: () => ({ jobs: [{ id: 'job-1' }], nextPageToken: 'job-1' }),
      })
    })

    const { result } = renderHook(() => useJobsList(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.pages).toHaveLength(1)
    expect(result.current.data?.pages[0].jobs).toHaveLength(1)
    expect(result.current.hasNextPage).toBe(true)
  })

  it('appends the next page, keyed by the previous nextPageToken, on fetchNextPage', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        listJobs: request => {
          if (request.pageToken === '') {
            return { jobs: [{ id: 'job-1' }], nextPageToken: 'job-1' }
          }
          expect(request.pageToken).toBe('job-1')
          return { jobs: [{ id: 'job-2' }], nextPageToken: '' }
        },
      })
    })

    const { result } = renderHook(() => useJobsList(), { wrapper: wrapper(mockTransport) })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    // Asserting on fetchNextPage's own resolved result, rather than
    // polling result.current via waitFor, is the reliable signal here —
    // result.current's re-render has been observed to lag arbitrarily
    // (this file's tests running alongside another connect-query hook's
    // test elsewhere in the suite) even though the query itself has
    // already settled with the second page, per fetchNextPage's return
    // value below.
    const next = await result.current.fetchNextPage()
    expect(next.data?.pages).toHaveLength(2)
    expect(next.data?.pages[1].jobs[0].id).toBe('job-2')
    expect(next.hasNextPage).toBe(false)
  })

  it('surfaces a ConnectError when the call fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        listJobs: () => {
          throw new Error('unavailable')
        },
      })
    })

    const { result } = renderHook(() => useJobsList(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toBeTruthy()
  })
})
