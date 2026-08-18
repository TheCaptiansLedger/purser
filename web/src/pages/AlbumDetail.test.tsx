import { ConnectError, Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { EntityType } from '../gen/purser/domain/v1/common_pb'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import { ImageBlobService } from '../gen/purser/domain/v1/image_blob_pb'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { TagService } from '../gen/purser/domain/v1/tag_pb'
import { TagAssignmentService } from '../gen/purser/domain/v1/tag_assignment_pb'
import { MusicReleaseService, ReleaseStatus } from '../gen/purser/music/v1/release_pb'
import { AlbumDetail } from './AlbumDetail'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern ArtistDetail.test.tsx
// uses.
function renderAlbumDetail(mockTransport: ReturnType<typeof createRouterTransport>, id = 'group-1') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[`/music/albums/${id}`]}>
          <Routes>
            <Route path="/music/albums/:id" element={<AlbumDetail />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </TransportProvider>,
  )
}

const baseGroup = { id: 'group-1', libraryEntryId: 'artist-1', title: 'Rumours', year: 1977 }

function noSelectedImage() {
  return () => {
    throw new ConnectError('not found', Code.NotFound)
  }
}

function registerNoTags(router: import('@connectrpc/connect').ConnectRouter) {
  router.service(TagAssignmentService, { listTagAssignments: () => ({ tagAssignments: [] }) })
}

function registerNoReleases(router: import('@connectrpc/connect').ConnectRouter) {
  router.service(MusicReleaseService, { listMusicReleases: () => ({ musicReleases: [] }) })
}

describe('AlbumDetail', () => {
  it("renders the Group's title and Hero facts from the default edition", async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { getGroup: () => ({ group: baseGroup }) })
      router.service(MusicReleaseService, {
        listMusicReleases: () => ({
          musicReleases: [
            { id: 'rel-1', groupId: 'group-1', isDefault: false, trackCount: 3, status: ReleaseStatus.PARTIAL },
            { id: 'rel-2', groupId: 'group-1', isDefault: true, trackCount: 11, status: ReleaseStatus.IMPORTED },
          ],
        }),
      })
      router.service(ImageService, { getSelectedImage: noSelectedImage() })
      registerNoTags(router)
    })

    renderAlbumDetail(mockTransport)

    expect(await screen.findByRole('heading', { name: 'Rumours' })).toBeInTheDocument()
    expect(screen.getByText('1977')).toBeInTheDocument()
    expect(screen.getByText('11 tracks')).toBeInTheDocument()
  })

  it('renders a Not found message when the Group is missing', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { getGroup: () => ({ group: undefined }) })
      registerNoReleases(router)
      registerNoTags(router)
    })

    renderAlbumDetail(mockTransport)

    expect(await screen.findByRole('alert')).toHaveTextContent('Album not found.')
  })

  it('renders no chip row when the Group has zero tag assignments', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { getGroup: () => ({ group: baseGroup }) })
      registerNoReleases(router)
      router.service(ImageService, { getSelectedImage: noSelectedImage() })
      registerNoTags(router)
    })

    renderAlbumDetail(mockTransport)

    expect(await screen.findByRole('heading', { name: 'Rumours' })).toBeInTheDocument()
    expect(screen.queryByText('Rock')).not.toBeInTheDocument()
  })

  it('renders resolved genre/mood tags as chips', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { getGroup: () => ({ group: baseGroup }) })
      registerNoReleases(router)
      router.service(ImageService, { getSelectedImage: noSelectedImage() })
      router.service(TagAssignmentService, {
        listTagAssignments: request => {
          expect(request.entityType).toBe(EntityType.GROUP)
          expect(request.entityId).toBe('group-1')
          return { tagAssignments: [{ tagId: 'tag-1', entityType: EntityType.GROUP, entityId: 'group-1' }] }
        },
      })
      router.service(TagService, { getTag: () => ({ tag: { id: 'tag-1', key: 'genre', value: 'Rock' } }) })
    })

    renderAlbumDetail(mockTransport)

    expect(await screen.findByText('Rock')).toBeInTheDocument()
  })

  it('hides the cover art controls when the Group has zero editions', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { getGroup: () => ({ group: baseGroup }) })
      registerNoReleases(router)
      registerNoTags(router)
    })

    renderAlbumDetail(mockTransport)

    expect(await screen.findByRole('heading', { name: 'Rumours' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Add cover' })).not.toBeInTheDocument()
  })

  it("attaches an uploaded cover to the default edition (owner_type=music_release), then opens it in the lightbox", async () => {
    let selectedCoverId: string | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { getGroup: () => ({ group: baseGroup }) })
      router.service(MusicReleaseService, {
        listMusicReleases: () => ({
          musicReleases: [{ id: 'rel-2', groupId: 'group-1', isDefault: true, trackCount: 11, status: ReleaseStatus.IMPORTED }],
        }),
      })
      router.service(ImageService, {
        getSelectedImage: req => {
          if (req.imageType === 'poster' && selectedCoverId) return { image: { id: selectedCoverId } }
          throw new ConnectError('not found', Code.NotFound)
        },
        createImage: req => {
          expect(req.image?.ownerType).toBe('music_release')
          expect(req.image?.ownerId).toBe('rel-2')
          expect(req.image?.imageType).toBe('poster')
          return { image: { ...req.image!, id: 'image-1' } }
        },
        selectImage: req => {
          selectedCoverId = req.imageId
          return { image: { id: req.imageId } }
        },
      })
      router.service(ImageBlobService, {
        uploadImage: () => ({
          blob: { key: 'music_release/ab/img-1.jpg', width: 300, height: 300, contentType: 'image/jpeg', sizeBytes: 1000n },
        }),
      })
      registerNoTags(router)
    })

    renderAlbumDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Rumours' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Add cover' }))
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File([new Uint8Array([1, 2, 3])], 'cover.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })

    const coverButton = await screen.findByRole('button', { name: "View Rumours's cover art" })
    expect(coverButton.querySelector('img')).toHaveAttribute('src', '/media/images/image-1')

    fireEvent.click(coverButton)
    expect(screen.getByRole('dialog', { name: 'Rumours' })).toBeInTheDocument()
  })
})
