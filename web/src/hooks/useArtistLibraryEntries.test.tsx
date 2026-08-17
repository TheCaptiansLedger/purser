import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { useArtistLibraryEntries } from './useArtistLibraryEntries'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same pattern as usePeopleList.test.tsx.
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

describe('useArtistLibraryEntries', () => {
  it('sends kind="artist" and fetches the first page', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        listLibraryEntries: request => {
          expect(request.kind).toBe('artist')
          return {
            libraryEntries: [{ id: 'a1', name: 'Fleetwood Mac' }],
            nextPageToken: 'a1',
          }
        },
      })
    })

    const { result } = renderHook(() => useArtistLibraryEntries(), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.pages[0].libraryEntries).toHaveLength(1)
    expect(result.current.hasNextPage).toBe(true)
  })

  it('appends the next page, keyed by the previous nextPageToken, on fetchNextPage', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        listLibraryEntries: request => {
          if (request.pageToken === '') {
            return { libraryEntries: [{ id: 'a1', name: 'Fleetwood Mac' }], nextPageToken: 'a1' }
          }
          expect(request.pageToken).toBe('a1')
          return { libraryEntries: [{ id: 'a2', name: 'Steely Dan' }], nextPageToken: '' }
        },
      })
    })

    const { result } = renderHook(() => useArtistLibraryEntries(), { wrapper: wrapper(mockTransport) })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    const next = await result.current.fetchNextPage()
    expect(next.data?.pages).toHaveLength(2)
    expect(next.data?.pages[1].libraryEntries[0].id).toBe('a2')
    expect(next.hasNextPage).toBe(false)
  })
})
