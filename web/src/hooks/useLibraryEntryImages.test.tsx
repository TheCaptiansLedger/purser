import { createRouterTransport } from '@connectrpc/connect'
import { ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { useLibraryEntryImages } from './useLibraryEntryImages'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// usePersonImages.test.tsx, the fan-out hook this one mirrors.
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

describe('useLibraryEntryImages', () => {
  it('resolves one imageId per library entry, keyed by id', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, {
        getSelectedImage: request => {
          expect(request.ownerType).toBe('library_entry')
          expect(request.imageType).toBe('poster')
          if (request.ownerId === 'a1') {
            return { image: { id: 'img-1' } }
          }
          return { image: { id: 'img-2' } }
        },
      })
    })

    const { result } = renderHook(() => useLibraryEntryImages(['a1', 'a2']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current).toEqual({ a1: 'img-1', a2: 'img-2' }))
  })

  it('omits entries with no selected image rather than mapping them to an empty id', async () => {
    const getSelectedImageSpy = vi.fn(() => {
      throw new ConnectError('not found', Code.NotFound)
    })
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, { getSelectedImage: getSelectedImageSpy })
    })

    const { result } = renderHook(() => useLibraryEntryImages(['a3']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(getSelectedImageSpy).toHaveBeenCalledOnce())
    expect(result.current).toEqual({})
  })
})
