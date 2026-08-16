import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { usePersonImages } from './usePersonImages'

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

describe('usePersonImages', () => {
  it('resolves one imageId per person, keyed by person id', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, {
        listImages: request => {
          expect(request.ownerType).toBe('person')
          if (request.ownerId === 'p1') {
            return { images: [{ id: 'img-1' }], nextPageToken: '' }
          }
          return { images: [{ id: 'img-2' }], nextPageToken: '' }
        },
      })
    })

    const { result } = renderHook(() => usePersonImages(['p1', 'p2']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current).toEqual({ p1: 'img-1', p2: 'img-2' }))
  })

  it('omits people with no image rather than mapping them to an empty id', async () => {
    const listImagesSpy = vi.fn(() => ({ images: [], nextPageToken: '' }))
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, { listImages: listImagesSpy })
    })

    const { result } = renderHook(() => usePersonImages(['p3']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(listImagesSpy).toHaveBeenCalledOnce())
    expect(result.current).toEqual({})
  })
})
