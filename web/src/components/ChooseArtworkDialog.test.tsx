import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ComponentProps } from 'react'
import { ImageBlobService } from '../gen/purser/domain/v1/image_blob_pb'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { ChooseArtworkDialog } from './ChooseArtworkDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// useAttachImage.test.tsx, which this component's dialog is built on.
function renderDialog(
  mockTransport: ReturnType<typeof createRouterTransport>,
  props: Partial<ComponentProps<typeof ChooseArtworkDialog>> = {},
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAttached = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <ChooseArtworkDialog
          title="Choose artwork"
          ownerType="person"
          ownerId="person-1"
          imageType="photo"
          onClose={onClose}
          onAttached={onAttached}
          {...props}
        />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAttached }
}

// Person photo — exercises the provider-pick entry point (ADR 0004:
// at least two content-type/prop configurations for a shared component).
describe('ChooseArtworkDialog — person photo, provider candidates', () => {
  const candidates = [{ url: 'https://fanart.tv/poster.jpg', source: 'fanart.tv', label: 'Poster' }]

  it('attaches the picked candidate and closes', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        cacheRemoteImage: () => ({
          blob: { key: 'person/ab/img-1.jpg', width: 300, height: 300, contentType: 'image/jpeg', sizeBytes: 1000n },
        }),
      })
      router.service(ImageService, {
        createImage: req => {
          expect(req.image?.ownerType).toBe('person')
          expect(req.image?.source).toBe('fanart.tv')
          return { image: { ...req.image!, id: 'image-1' } }
        },
        selectImage: req => {
          expect(req.imageId).toBe('image-1')
          return { image: { id: 'image-1' } }
        },
      })
    })
    const { onClose, onAttached } = renderDialog(mockTransport, { candidates })

    fireEvent.click(screen.getByRole('button', { name: 'Poster' }))

    await waitFor(() => expect(onAttached).toHaveBeenCalledWith(expect.objectContaining({ id: 'image-1' })))
    expect(onClose).toHaveBeenCalled()
  })

  it("shows a fetch error and doesn't close when CacheRemoteImage fails", async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        cacheRemoteImage: () => {
          throw new Error('remote fetch failed')
        },
      })
    })
    const { onClose, onAttached } = renderDialog(mockTransport, { candidates })

    fireEvent.click(screen.getByRole('button', { name: 'Poster' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't fetch that image"))
    expect(onAttached).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows an attach error, distinct from a fetch error, when CreateImage fails after a successful cache', async () => {
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
    const { onClose, onAttached } = renderDialog(mockTransport, { candidates })

    fireEvent.click(screen.getByRole('button', { name: 'Poster' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("was stored but couldn't be attached"))
    expect(onAttached).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows a select error, distinct from an attach error, when SelectImage fails after a successful create', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        cacheRemoteImage: () => ({
          blob: { key: 'person/ab/img-1.jpg', width: 300, height: 300, contentType: 'image/jpeg', sizeBytes: 1000n },
        }),
      })
      router.service(ImageService, {
        createImage: req => ({ image: { ...req.image!, id: 'image-1' } }),
        selectImage: () => {
          throw new Error('slot locked')
        },
      })
    })
    const { onClose, onAttached } = renderDialog(mockTransport, { candidates })

    fireEvent.click(screen.getByRole('button', { name: 'Poster' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("attached but couldn't be set as current"))
    expect(onAttached).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})

// Music album poster, upload-only — the second content-type/prop
// configuration, and the file-upload entry point.
describe('ChooseArtworkDialog — group poster, upload only', () => {
  const uploadProps = { ownerType: 'group', ownerId: 'group-1', imageType: 'poster' }

  function pickFile() {
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File([new Uint8Array([1, 2, 3])], 'cover.jpg', { type: 'image/jpeg' })
    fireEvent.change(input, { target: { files: [file] } })
  }

  it('attaches the uploaded file and closes', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        uploadImage: () => ({
          blob: { key: 'group/cd/img-2.jpg', width: 200, height: 200, contentType: 'image/jpeg', sizeBytes: 3n },
        }),
      })
      router.service(ImageService, {
        createImage: req => {
          expect(req.image?.ownerType).toBe('group')
          expect(req.image?.source).toBe('user')
          return { image: { ...req.image!, id: 'image-2' } }
        },
        selectImage: req => {
          expect(req.imageId).toBe('image-2')
          return { image: { id: 'image-2' } }
        },
      })
    })
    const { onClose, onAttached } = renderDialog(mockTransport, uploadProps)

    pickFile()

    await waitFor(() => expect(onAttached).toHaveBeenCalledWith(expect.objectContaining({ id: 'image-2' })))
    expect(onClose).toHaveBeenCalled()
  })

  it("shows a fetch error and doesn't close when UploadImage fails", async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        uploadImage: () => {
          throw new Error('file too large')
        },
      })
    })
    const { onClose, onAttached } = renderDialog(mockTransport, uploadProps)

    pickFile()

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't fetch that image"))
    expect(onAttached).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows an attach error, distinct from a fetch error, when CreateImage fails after a successful upload', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageBlobService, {
        uploadImage: () => ({
          blob: { key: 'group/cd/img-2.jpg', width: 200, height: 200, contentType: 'image/jpeg', sizeBytes: 3n },
        }),
      })
      router.service(ImageService, {
        createImage: () => {
          throw new Error('invalid owner_id')
        },
      })
    })
    const { onClose, onAttached } = renderDialog(mockTransport, uploadProps)

    pickFile()

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("was stored but couldn't be attached"))
    expect(onAttached).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
