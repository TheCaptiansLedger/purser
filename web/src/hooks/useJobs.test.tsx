import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { JobService } from '../gen/purser/job/v1/job_pb'
import { useJobs } from './useJobs'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend. createRouterTransport is Connect's own in-memory Transport
// implementation for exactly this — no real socket, not even loopback.
function wrapper(mockTransport: ReturnType<typeof createRouterTransport>) {
  // retry: false — otherwise TanStack Query's default retry/backoff means
  // the error-path test below wouldn't settle into isError within a sane
  // waitFor timeout.
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <TransportProvider transport={mockTransport}>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </TransportProvider>
    )
  }
}

describe('useJobs', () => {
  it('returns jobs from a successful ListJobs call', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        listJobs: () => ({ jobs: [{ id: 'job-1' }], nextPageToken: '' }),
      })
    })

    const { result } = renderHook(() => useJobs(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.jobs).toHaveLength(1)
    expect(result.current.data?.nextPageToken).toBe('')
  })

  it('surfaces a ConnectError when the call fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(JobService, {
        listJobs: () => {
          throw new Error('unavailable')
        },
      })
    })

    const { result } = renderHook(() => useJobs(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toBeTruthy()
  })
})
