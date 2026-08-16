import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { CacheService } from '../gen/purser/cache/v1/cache_pb'
import { useFlushCacheMutation } from './useFlushCacheMutation'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useSettings.test.tsx.
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

describe('useFlushCacheMutation', () => {
  it('sends the name the caller passes and returns the flushed list', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(CacheService, {
        flushCache: req => {
          expect(req.name).toBe('musicbrainz')
          return { flushed: ['musicbrainz'] }
        },
      })
    })

    const { result } = renderHook(() => useFlushCacheMutation(), { wrapper: wrapper(mockTransport) })

    result.current.mutate({ name: 'musicbrainz' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.flushed).toEqual(['musicbrainz'])
  })

  it('flushes every registered cache when name is empty', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(CacheService, {
        flushCache: () => ({ flushed: ['musicbrainz', 'stashdb'] }),
      })
    })

    const { result } = renderHook(() => useFlushCacheMutation(), { wrapper: wrapper(mockTransport) })

    result.current.mutate({ name: '' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.flushed).toEqual(['musicbrainz', 'stashdb'])
  })
})
