import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { CacheService } from '../gen/purser/cache/v1/cache_pb'
import { useCacheStats } from './useCacheStats'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useDatabaseInfo.test.tsx.
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

describe('useCacheStats', () => {
  it('returns cache stats from a successful ListCacheStats call', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(CacheService, {
        listCacheStats: () => ({
          caches: [{ name: 'musicbrainz', items: 12, bytes: 4096n, hits: 80n, misses: 20n }],
        }),
      })
    })

    const { result } = renderHook(() => useCacheStats(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.caches[0].name).toBe('musicbrainz')
  })

  it('surfaces a ConnectError when ListCacheStats fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(CacheService, {
        listCacheStats: () => {
          throw new Error('unavailable')
        },
      })
    })

    const { result } = renderHook(() => useCacheStats(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toBeTruthy()
  })
})
