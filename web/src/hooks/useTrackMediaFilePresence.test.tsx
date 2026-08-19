import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import { MediaFileService } from '../gen/purser/domain/v1/media_file_pb'
import type { Item } from '../gen/purser/domain/v1/item_pb'
import { useTrackMediaFilePresence } from './useTrackMediaFilePresence'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend.
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

function track(id: string, status: ItemStatus): Item {
  return { id, status } as Item
}

describe('useTrackMediaFilePresence', () => {
  it('reports which imported tracks actually have a MediaFile', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MediaFileService, {
        listMediaFiles: req => ({
          mediaFiles: req.itemId === 'imported-with-file' ? [{ id: 'mf-1' }] : [],
        }),
      })
    })

    const tracks = [
      track('imported-with-file', ItemStatus.IMPORTED),
      track('imported-without-file', ItemStatus.IMPORTED),
      track('missing-track', ItemStatus.MISSING),
    ]

    const { result } = renderHook(() => useTrackMediaFilePresence(tracks), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.has('imported-with-file')).toBe(true))
    expect(result.current.has('imported-without-file')).toBe(false)
  })

  it('never queries a non-imported track — bounded fan-out over imported tracks only', () => {
    const queried: string[] = []
    const mockTransport = createRouterTransport(router => {
      router.service(MediaFileService, {
        listMediaFiles: req => {
          queried.push(req.itemId)
          return { mediaFiles: [] }
        },
      })
    })

    const tracks = [track('wanted-track', ItemStatus.WANTED), track('missing-track', ItemStatus.MISSING)]

    renderHook(() => useTrackMediaFilePresence(tracks), { wrapper: wrapper(mockTransport) })

    expect(queried).toEqual([])
  })
})
