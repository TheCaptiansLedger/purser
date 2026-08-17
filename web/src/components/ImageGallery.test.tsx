import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, afterEach } from 'vitest'
import type { ComponentProps } from 'react'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { ImageGallery } from './ImageGallery'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// ChooseArtworkDialog.test.tsx, this component's counterpart on the
// manage-existing-images side of the same flow.
function renderGallery(
  mockTransport: ReturnType<typeof createRouterTransport>,
  props: Partial<ComponentProps<typeof ImageGallery>> = {},
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onChange = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <ImageGallery
          title="Manage photos"
          ownerType="person"
          ownerId="person-1"
          imageType="photo"
          onClose={onClose}
          onChange={onChange}
          {...props}
        />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onChange }
}

afterEach(() => {
  vi.restoreAllMocks()
})

// Person photo, two attached images — the first content-type/prop
// configuration (ADR 0004: at least two).
describe('ImageGallery — person photo, two attached images', () => {
  function twoImageTransport() {
    return createRouterTransport(router => {
      router.service(ImageService, {
        listImages: () => ({
          images: [{ id: 'img-2' }, { id: 'img-1' }],
          nextPageToken: '',
        }),
        getSelectedImage: () => ({ image: { id: 'img-1' } }),
      })
    })
  }

  it('marks the selected image and offers "Use this one" only on the others', async () => {
    renderGallery(twoImageTransport())

    await waitFor(() => expect(screen.getByText('Current')).toBeInTheDocument())
    expect(screen.getAllByRole('button', { name: 'Use this one' })).toHaveLength(1)
  })

  it('selects a different image and reports the change', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, {
        listImages: () => ({ images: [{ id: 'img-2' }, { id: 'img-1' }], nextPageToken: '' }),
        getSelectedImage: () => ({ image: { id: 'img-1' } }),
        selectImage: req => {
          expect(req.imageId).toBe('img-2')
          return { image: { id: 'img-2' } }
        },
      })
    })
    const { onChange } = renderGallery(mockTransport)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Use this one' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Use this one' }))

    await waitFor(() => expect(onChange).toHaveBeenCalled())
  })

  it('deletes an image after confirmation and reports the change', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, {
        listImages: () => ({ images: [{ id: 'img-2' }, { id: 'img-1' }], nextPageToken: '' }),
        getSelectedImage: () => ({ image: { id: 'img-1' } }),
        deleteImage: req => {
          expect(req.id).toBe('img-2')
          return {}
        },
      })
    })
    const { onChange } = renderGallery(mockTransport)

    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Delete image' })).toHaveLength(2))
    fireEvent.click(screen.getAllByRole('button', { name: 'Delete image' })[0])

    await waitFor(() => expect(onChange).toHaveBeenCalled())
  })

  it("doesn't delete when the confirmation is declined", async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const deleteSpy = vi.fn(() => ({}))
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, {
        listImages: () => ({ images: [{ id: 'img-1' }], nextPageToken: '' }),
        getSelectedImage: () => ({ image: { id: 'img-1' } }),
        deleteImage: deleteSpy,
      })
    })
    renderGallery(mockTransport)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete image' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Delete image' }))

    expect(deleteSpy).not.toHaveBeenCalled()
  })
})

// Group poster, empty and error states — the second content-type/prop
// configuration.
describe('ImageGallery — group poster, empty and error states', () => {
  it('shows an empty state when nothing has been attached', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, {
        listImages: () => ({ images: [], nextPageToken: '' }),
        getSelectedImage: () => {
          throw new Error('not found')
        },
      })
    })
    renderGallery(mockTransport, { ownerType: 'group', ownerId: 'group-1', imageType: 'poster' })

    await waitFor(() => expect(screen.getByText('No images yet')).toBeInTheDocument())
  })

  it('shows an error when ListImages fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ImageService, {
        listImages: () => {
          throw new Error('unavailable')
        },
      })
    })
    renderGallery(mockTransport, { ownerType: 'group', ownerId: 'group-1', imageType: 'poster' })

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load images"))
  })
})
