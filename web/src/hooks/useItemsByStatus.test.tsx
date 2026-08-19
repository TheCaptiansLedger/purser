import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { ItemService } from '../gen/purser/domain/v1/item_pb'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import { useItemsByStatus } from './useItemsByStatus'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same pattern as useArtistLibraryEntries.test.tsx, the
// infinite-query hook this one mirrors.
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

describe('useItemsByStatus', () => {
  it('sends content_type="music" and the given status, and fetches the first page', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        listItems: request => {
          expect(request.contentType).toBe('music')
          expect(request.status).toBe(ItemStatus.WANTED)
          return { items: [{ id: 'i1', title: 'Dreams' }], nextPageToken: 'i1' }
        },
      })
    })

    const { result } = renderHook(() => useItemsByStatus(ItemStatus.WANTED), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.pages[0].items).toHaveLength(1)
    expect(result.current.hasNextPage).toBe(true)
  })

  it('appends the next page, keyed by the previous nextPageToken, on fetchNextPage', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        listItems: request => {
          if (request.pageToken === '') {
            return { items: [{ id: 'i1', title: 'Dreams' }], nextPageToken: 'i1' }
          }
          expect(request.pageToken).toBe('i1')
          return { items: [{ id: 'i2', title: 'Gold Dust Woman' }], nextPageToken: '' }
        },
      })
    })

    const { result } = renderHook(() => useItemsByStatus(ItemStatus.MISSING), { wrapper: wrapper(mockTransport) })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    const next = await result.current.fetchNextPage()
    expect(next.data?.pages).toHaveLength(2)
    expect(next.data?.pages[1].items[0].id).toBe('i2')
    expect(next.hasNextPage).toBe(false)
  })
})
