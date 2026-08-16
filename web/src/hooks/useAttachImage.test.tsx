import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { ImageBlobService } from '../gen/purser/domain/v1/image_blob_pb'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { useAttachImage } from './useAttachImage'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useSettings.test.tsx.
// Both services this hook composes are registered on one router, same as
// a real server would expose them both.
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

const target = { ownerType: 'person', ownerId: 'person-1', imageType: 'photo' }

describe('useAttachImage.attachFromUrl', () => {
  it('caches the remote URL then creates the Image row', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        cacheRemoteImage: req => {
          expect(req.url).toBe('https://fanart.tv/poster.jpg')
          return { blob: { key: 'person/ab/img-1.jpg', width: 300, height: 300, contentType: 'image/jpeg', sizeBytes: 1000n } }
        },
      })
      router.service(ImageService, {
        createImage: req => {
          expect(req.image?.url).toBe('person/ab/img-1.jpg')
          expect(req.image?.source).toBe('fanart.tv')
          return { image: { ...req.image!, id: 'image-1' } }
        },
      })
    })

    const { result } = renderHook(() => useAttachImage(), { wrapper: wrapper(mockTransport) })

    let image
    await act(async () => {
      image = await result.current.attachFromUrl({ url: 'https://fanart.tv/poster.jpg', source: 'fanart.tv', ...target })
    })

    expect(image).toMatchObject({ id: 'image-1', url: 'person/ab/img-1.jpg' })
    await waitFor(() => expect(result.current.cacheRemoteImage.isSuccess).toBe(true))
    await waitFor(() => expect(result.current.createImage.isSuccess).toBe(true))
  })

  it('surfaces a CacheRemoteImage failure on cacheRemoteImage state and never calls CreateImage', async () => {
    let createImageCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        cacheRemoteImage: () => {
          throw new Error('fetch failed: 404')
        },
      })
      router.service(ImageService, {
        createImage: req => {
          createImageCalled = true
          return { image: { ...req.image!, id: 'image-1' } }
        },
      })
    })

    const { result } = renderHook(() => useAttachImage(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await expect(
        result.current.attachFromUrl({ url: 'https://fanart.tv/missing.jpg', source: 'fanart.tv', ...target }),
      ).rejects.toThrow()
    })

    await waitFor(() => expect(result.current.cacheRemoteImage.isError).toBe(true))
    expect(result.current.createImage.isError).toBe(false)
    expect(result.current.createImage.isSuccess).toBe(false)
    expect(createImageCalled).toBe(false)
  })

  it('surfaces a CreateImage failure on createImage state after a successful cache', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        cacheRemoteImage: () => ({
          blob: { key: 'person/ab/img-1.jpg', width: 300, height: 300, contentType: 'image/jpeg', sizeBytes: 1000n },
        }),
      })
      router.service(ImageService, {
        createImage: () => {
          throw new Error('invalid owner_id')
        },
      })
    })

    const { result } = renderHook(() => useAttachImage(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await expect(
        result.current.attachFromUrl({ url: 'https://fanart.tv/poster.jpg', source: 'fanart.tv', ...target }),
      ).rejects.toThrow()
    })

    await waitFor(() => expect(result.current.cacheRemoteImage.isSuccess).toBe(true))
    await waitFor(() => expect(result.current.createImage.isError).toBe(true))
  })
})

describe('useAttachImage.attachFromFile', () => {
  it('uploads the file then creates the Image row with source "user"', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        uploadImage: req => {
          expect(req.data).toEqual(new Uint8Array([1, 2, 3]))
          return { blob: { key: 'person/cd/img-2.jpg', width: 200, height: 200, contentType: 'image/jpeg', sizeBytes: 3n } }
        },
      })
      router.service(ImageService, {
        createImage: req => {
          expect(req.image?.source).toBe('user')
          return { image: { ...req.image!, id: 'image-2' } }
        },
      })
    })

    const { result } = renderHook(() => useAttachImage(), { wrapper: wrapper(mockTransport) })

    let image
    await act(async () => {
      image = await result.current.attachFromFile({ data: new Uint8Array([1, 2, 3]), ...target })
    })

    expect(image).toMatchObject({ id: 'image-2', url: 'person/cd/img-2.jpg' })
  })

  it('surfaces an UploadImage failure on uploadImage state and never calls CreateImage', async () => {
    let createImageCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        uploadImage: () => {
          throw new Error('file too large')
        },
      })
      router.service(ImageService, {
        createImage: req => {
          createImageCalled = true
          return { image: { ...req.image!, id: 'image-2' } }
        },
      })
    })

    const { result } = renderHook(() => useAttachImage(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await expect(result.current.attachFromFile({ data: new Uint8Array([1, 2, 3]), ...target })).rejects.toThrow()
    })

    await waitFor(() => expect(result.current.uploadImage.isError).toBe(true))
    expect(result.current.createImage.isSuccess).toBe(false)
    expect(createImageCalled).toBe(false)
  })

  it('surfaces a CreateImage failure on createImage state after a successful upload', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        uploadImage: () => ({
          blob: { key: 'person/cd/img-2.jpg', width: 200, height: 200, contentType: 'image/jpeg', sizeBytes: 3n },
        }),
      })
      router.service(ImageService, {
        createImage: () => {
          throw new Error('invalid owner_id')
        },
      })
    })

    const { result } = renderHook(() => useAttachImage(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await expect(result.current.attachFromFile({ data: new Uint8Array([1, 2, 3]), ...target })).rejects.toThrow()
    })

    await waitFor(() => expect(result.current.uploadImage.isSuccess).toBe(true))
    await waitFor(() => expect(result.current.createImage.isError).toBe(true))
  })
})
