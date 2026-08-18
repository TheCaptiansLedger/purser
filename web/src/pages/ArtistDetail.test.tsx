import type { ConnectRouter } from '@connectrpc/connect'
import { ConnectError, Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import { ImageBlobService } from '../gen/purser/domain/v1/image_blob_pb'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { FanartTVService } from '../gen/purser/music/v1/fanarttv_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { MusicReleaseService, ReleaseStatus } from '../gen/purser/music/v1/release_pb'
import { TheAudioDBService } from '../gen/purser/music/v1/theaudiodb_pb'
import { ArtistDetail } from './ArtistDetail'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as PersonDetail.test.tsx,
// which the poster-attach/lightbox assertions below reuse unmodified
// (composition, not a re-test of ChooseArtworkDialog/ImageGallery's own
// error states — those already have their own tests).
function renderArtistDetail(mockTransport: ReturnType<typeof createRouterTransport>, id = 'artist-1') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[`/music/artists/${id}`]}>
          <Routes>
            <Route path="/music/artists/:id" element={<ArtistDetail />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </TransportProvider>,
  )
}

const baseEntry = {
  id: 'artist-1',
  contentType: 'music',
  kind: 'artist',
  name: 'Fleetwood Mac',
  sortName: 'Fleetwood Mac',
  monitored: false,
  monitorMode: MonitorMode.NONE,
  metadata: {
    genre: 'Rock',
    style: 'Soft Rock',
    artist_type: 'Group',
    country: 'GB',
    founded_date: '1967',
    aliases: ['FM'],
    isni: '0000000123456789',
    official_url: 'https://fleetwoodmac.com/',
    wikipedia_url: 'https://en.wikipedia.org/wiki/Fleetwood_Mac',
  },
}

function noExternalID() {
  return () => {
    throw new ConnectError('not found', Code.NotFound)
  }
}

function noSelectedImage() {
  return () => {
    throw new ConnectError('not found', Code.NotFound)
  }
}

// registerNoProviderData — the mbz ExternalID lookup 404s, so
// useArtistProviderData never calls fanart.tv/TheAudioDB at all; every
// test that renders past GetLibraryEntry but isn't exercising the
// provider fan-out itself uses this to stay focused on what it asserts.
function registerNoProviderData(router: ConnectRouter) {
  router.service(ExternalIDService, { getExternalID: noExternalID() })
}

function registerNoImages(router: ConnectRouter) {
  router.service(ImageService, { getSelectedImage: noSelectedImage() })
}

// registerNoAlbums — every test not specifically exercising the
// Discography tab registers this so useDiscography's unconditional
// ListGroups call resolves to an empty page rather than hitting an
// unregistered service.
function registerNoAlbums(router: ConnectRouter) {
  router.service(GroupService, { listGroups: () => ({ groups: [] }) })
}

// registerNoMembers — every test not specifically exercising the Members
// tab registers this so useArtistMembers' unconditional ListEntryPeople
// call resolves to an empty page rather than hitting an unregistered
// service, same precedent registerNoAlbums already set for useDiscography.
function registerNoMembers(router: ConnectRouter) {
  router.service(EntryPersonService, { listEntryPeople: () => ({ entryPeople: [] }) })
}

describe('ArtistDetail — load', () => {
  it('renders the Hero and facts sidebar from GetLibraryEntry', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())
    expect(screen.getByText('Rock')).toBeInTheDocument()
    expect(screen.getByText('Soft Rock')).toBeInTheDocument()
    expect(screen.getByText('Group')).toBeInTheDocument()
    expect(screen.getByText('GB')).toBeInTheDocument()
    expect(screen.getByText('Since 1967')).toBeInTheDocument()
    expect(screen.getByText('ISNI 0000000123456789')).toBeInTheDocument()
    expect(screen.getByText('Also known as FM')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Official site' })).toHaveAttribute('href', 'https://fleetwoodmac.com/')
    expect(screen.getByRole('link', { name: 'Wikipedia' })).toHaveAttribute(
      'href',
      'https://en.wikipedia.org/wiki/Fleetwood_Mac',
    )
  })

  it('omits the ISNI/official-site/Wikipedia rows entirely when Metadata carries none of them', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntry: () => ({ libraryEntry: { ...baseEntry, metadata: { artist_type: 'Group' } } }),
      })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())
    expect(screen.queryByText(/^ISNI /)).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Official site' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Wikipedia' })).not.toBeInTheDocument()
  })

  it('shows an actionable error when GetLibraryEntry fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntry: () => {
          throw new Error('not found')
        },
      })
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load this artist"))
  })

  it('renders without bio/backdrop when there is no mbz ExternalID row — graceful partial data', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())
    expect(screen.queryByRole('img', { name: '' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'View backdrop full-screen' })).not.toBeInTheDocument()
  })

  it('renders without bio/backdrop when the mbid lookup 404s against both providers', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      router.service(ExternalIDService, { getExternalID: () => ({ externalId: { value: 'mbid-1' } }) })
      router.service(FanartTVService, {
        lookupArtist: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
      router.service(TheAudioDBService, {
        lookupArtist: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'View backdrop full-screen' })).not.toBeInTheDocument()
  })
})

describe('ArtistDetail — monitor toggle round-trip', () => {
  it('flips optimistically on click, then reconciles with the UpdateLibraryEntry response', async () => {
    let resolveUpdate: (() => void) | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntry: () => ({ libraryEntry: baseEntry }),
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['monitored', 'monitor_mode'])
          return new Promise(resolve => {
            resolveUpdate = () =>
              resolve({ libraryEntry: { ...baseEntry, monitored: true, monitorMode: MonitorMode.ALL } })
          })
        },
      })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    const toggle = screen.getByRole('switch', { name: 'Monitored' })
    expect(toggle).toHaveAttribute('aria-checked', 'false')

    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'true')

    resolveUpdate?.()
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'true'))
  })

  it('rolls back to the previous state when UpdateLibraryEntry fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntry: () => ({ libraryEntry: baseEntry }),
        updateLibraryEntry: () => {
          throw new Error('unavailable')
        },
      })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    const toggle = screen.getByRole('switch', { name: 'Monitored' })
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'true')

    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'false'))
  })
})

describe('ArtistDetail — poster attach and lightbox', () => {
  it('attaches an uploaded poster via ChooseArtworkDialog (which also selects it), then opens it in the lightbox on click', async () => {
    let selectedPosterId: string | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      router.service(ImageService, {
        getSelectedImage: req => {
          if (req.imageType === 'poster' && selectedPosterId) return { image: { id: selectedPosterId } }
          throw new ConnectError('not found', Code.NotFound)
        },
        createImage: req => {
          expect(req.image?.ownerType).toBe('library_entry')
          expect(req.image?.ownerId).toBe('artist-1')
          expect(req.image?.imageType).toBe('poster')
          return { image: { ...req.image!, id: 'image-1' } }
        },
        selectImage: req => {
          selectedPosterId = req.imageId
          return { image: { id: req.imageId } }
        },
      })
      router.service(ImageBlobService, {
        uploadImage: () => ({
          blob: { key: 'library_entry/ab/img-1.jpg', width: 300, height: 450, contentType: 'image/jpeg', sizeBytes: 1000n },
        }),
      })
      registerNoProviderData(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Add poster' }))
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File([new Uint8Array([1, 2, 3])], 'poster.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })

    const posterButton = await screen.findByRole('button', { name: "View Fleetwood Mac's poster" })
    expect(posterButton.querySelector('img')).toHaveAttribute('src', '/media/images/image-1')

    fireEvent.click(posterButton)
    expect(screen.getByRole('dialog', { name: 'Fleetwood Mac' })).toBeInTheDocument()
  })
})

describe('ArtistDetail — backdrop attach from a fanart.tv candidate', () => {
  it('offers artist_background and hd_music_logo as candidates, attaches the picked one, and it becomes the Hero backdrop', async () => {
    let selectedBackdropId: string | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      router.service(ExternalIDService, { getExternalID: () => ({ externalId: { value: 'mbid-1' } }) })
      router.service(FanartTVService, {
        lookupArtist: () => ({
          artist: {
            artistBackground: [{ id: '1', url: 'https://fanart/bg.jpg', likes: '1', lang: '' }],
            hdMusicLogo: [{ id: '2', url: 'https://fanart/logo.png', likes: '1', lang: '' }],
          },
        }),
      })
      router.service(TheAudioDBService, { lookupArtist: () => ({ artist: { biography: 'A band from London.' } }) })
      router.service(ImageService, {
        getSelectedImage: req => {
          if (req.imageType === 'backdrop' && selectedBackdropId) return { image: { id: selectedBackdropId } }
          throw new ConnectError('not found', Code.NotFound)
        },
        createImage: req => {
          expect(req.image?.ownerType).toBe('library_entry')
          expect(req.image?.imageType).toBe('backdrop')
          expect(req.image?.source).toBe('fanart.tv')
          return { image: { ...req.image!, id: 'image-2' } }
        },
        selectImage: req => {
          selectedBackdropId = req.imageId
          return { image: { id: req.imageId } }
        },
      })
      router.service(ImageBlobService, {
        cacheRemoteImage: () => ({
          blob: { key: 'library_entry/cd/img-2.jpg', width: 1920, height: 1080, contentType: 'image/jpeg', sizeBytes: 2000n },
        }),
      })
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByText('A band from London.')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Add backdrop' }))
    expect(await screen.findByRole('button', { name: 'Background' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Logo' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Background' }))

    const viewButton = await screen.findByRole('button', { name: 'View backdrop full-screen' })
    fireEvent.click(viewButton)
    expect(screen.getByRole('dialog', { name: 'Fleetwood Mac backdrop' })).toBeInTheDocument()
  })
})

describe('ArtistDetail — poster attach from a fanart.tv candidate', () => {
  it('offers artist_thumb as a poster candidate and attaches the picked one', async () => {
    let selectedPosterId: string | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      router.service(ExternalIDService, { getExternalID: () => ({ externalId: { value: 'mbid-1' } }) })
      router.service(FanartTVService, {
        lookupArtist: () => ({
          artist: { artistThumb: [{ id: '3', url: 'https://fanart/thumb.jpg', likes: '1', lang: '' }] },
        }),
      })
      router.service(TheAudioDBService, { lookupArtist: () => ({ artist: {} }) })
      router.service(ImageService, {
        getSelectedImage: req => {
          if (req.imageType === 'poster' && selectedPosterId) return { image: { id: selectedPosterId } }
          throw new ConnectError('not found', Code.NotFound)
        },
        createImage: req => {
          expect(req.image?.imageType).toBe('poster')
          expect(req.image?.source).toBe('fanart.tv')
          return { image: { ...req.image!, id: 'image-3' } }
        },
        selectImage: req => {
          selectedPosterId = req.imageId
          return { image: { id: req.imageId } }
        },
      })
      router.service(ImageBlobService, {
        cacheRemoteImage: () => ({
          blob: { key: 'library_entry/ef/img-3.jpg', width: 1000, height: 1500, contentType: 'image/jpeg', sizeBytes: 1500n },
        }),
      })
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Add poster' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Thumb' }))

    const posterButton = await screen.findByRole('button', { name: "View Fleetwood Mac's poster" })
    expect(posterButton.querySelector('img')).toHaveAttribute('src', '/media/images/image-3')
  })
})

describe('ArtistDetail — Discography tab', () => {
  it('shows an empty state when the artist has no groups', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    expect(await screen.findByText('No albums yet')).toBeInTheDocument()
    // No mbz ExternalID row (registerNoProviderData) means no known MBID
    // to browse a discography by — ListReleaseGroupsForArtist has no
    // non-MBID fallback, unlike the poster/backdrop buttons. "Add
    // Manually" has no such dependency and stays enabled.
    fireEvent.click(screen.getByRole('button', { name: 'Add Album' }))
    expect(screen.getByRole('menuitem', { name: 'Search MusicBrainz' })).toBeDisabled()
    expect(screen.getByRole('menuitem', { name: 'Add Manually' })).toBeEnabled()
  })

  it('adds an album via Add Album → Search MusicBrainz and refreshes the grid with it, without navigating away', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      router.service(ExternalIDService, {
        getExternalID: () => ({ externalId: { value: 'mbid-1' } }),
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(FanartTVService, {
        lookupArtist: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
      router.service(TheAudioDBService, {
        lookupArtist: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
      router.service(MusicBrainzService, {
        listReleaseGroupsForArtist: req => {
          expect(req.artistMbid).toBe('mbid-1')
          return { releaseGroups: [{ mbid: 'rg-1', title: 'Rumours', primaryType: 'Album' }] }
        },
      })
      registerNoImages(router)
      registerNoMembers(router)

      let groupsAdded = false
      router.service(GroupService, {
        listGroups: () => ({ groups: groupsAdded ? [{ id: 'group-1', title: 'Rumours', year: 0 }] : [] }),
        createGroup: req => {
          expect(req.group?.libraryEntryId).toBe('artist-1')
          groupsAdded = true
          return { group: { id: 'group-1', title: 'Rumours' } }
        },
      })
      router.service(MusicReleaseService, { listMusicReleases: () => ({ musicReleases: [] }) })
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    expect(await screen.findByText('No albums yet')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Album' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Search MusicBrainz' }))
    fireEvent.click(await screen.findByRole('button', { name: /Rumours/ }))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Add Album' })).not.toBeInTheDocument())
    expect(await screen.findByText('Rumours')).toBeInTheDocument()
  })

  it('adds an album via Add Album → Add Manually and refreshes the grid with it', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoMembers(router)

      let groupsAdded = false
      router.service(GroupService, {
        listGroups: () => ({ groups: groupsAdded ? [{ id: 'group-1', title: 'Live Aid Bootleg' }] : [] }),
        createGroup: req => {
          expect(req.group?.libraryEntryId).toBe('artist-1')
          expect(req.group?.title).toBe('Live Aid Bootleg')
          groupsAdded = true
          return { group: { id: 'group-1', title: 'Live Aid Bootleg' } }
        },
      })
      router.service(MusicReleaseService, { listMusicReleases: () => ({ musicReleases: [] }) })
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    expect(await screen.findByText('No albums yet')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Album' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Add Manually' }))

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Add Album Manually' })).not.toBeInTheDocument())
    expect(await screen.findByText('Live Aid Bootleg')).toBeInTheDocument()
  })

  it("badges an album with its default edition's status", async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      router.service(GroupService, {
        listGroups: () => ({ groups: [{ id: 'group-1', title: 'Rumours', year: 1977 }] }),
      })
      router.service(MusicReleaseService, {
        listMusicReleases: () => ({
          musicReleases: [
            { id: 'rel-1', groupId: 'group-1', isDefault: false, status: ReleaseStatus.PARTIAL },
            { id: 'rel-2', groupId: 'group-1', isDefault: true, status: ReleaseStatus.IMPORTED },
          ],
        }),
      })
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    expect(await screen.findByText('Rumours')).toBeInTheDocument()
    expect(screen.getByText('1977')).toBeInTheDocument()
    expect(screen.getByText('Imported')).toBeInTheDocument()
  })

  it('falls back to the first returned edition when none is marked default', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      router.service(GroupService, {
        listGroups: () => ({ groups: [{ id: 'group-1', title: 'Tusk', year: 1979 }] }),
      })
      router.service(MusicReleaseService, {
        listMusicReleases: () => ({
          musicReleases: [{ id: 'rel-1', groupId: 'group-1', isDefault: false, status: ReleaseStatus.STUB }],
        }),
      })
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    expect(await screen.findByText('Tusk')).toBeInTheDocument()
    expect(screen.getByText('Stub')).toBeInTheDocument()
  })

  it('renders "No edition selected" for a Group with zero MusicRelease rows, without crashing', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      router.service(GroupService, {
        listGroups: () => ({ groups: [{ id: 'group-1', title: 'Unreleased Sessions', year: 0 }] }),
      })
      router.service(MusicReleaseService, { listMusicReleases: () => ({ musicReleases: [] }) })
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    expect(await screen.findByText('Unreleased Sessions')).toBeInTheDocument()
    expect(screen.getByText('No edition selected')).toBeInTheDocument()
  })
})

describe('ArtistDetail — Members tab', () => {
  it('shows an empty state when the artist has no members', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('tab', { name: 'Members' }))

    expect(await screen.findByText('No members yet')).toBeInTheDocument()
  })

  it('splits current and former members into separate sections, with era shown in the role chip', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      router.service(EntryPersonService, {
        listEntryPeople: () => ({
          entryPeople: [
            {
              libraryEntryId: 'artist-1',
              personId: 'person-1',
              role: 'vocalist',
              startDate: timestampFromDate(new Date('1975-01-01')),
            },
            {
              libraryEntryId: 'artist-1',
              personId: 'person-2',
              role: 'guitarist',
              startDate: timestampFromDate(new Date('1975-01-01')),
              endDate: timestampFromDate(new Date('1987-01-01')),
            },
          ],
        }),
      })
      router.service(PersonService, {
        getPerson: req => {
          const names: Record<string, string> = { 'person-1': 'Stevie Nicks', 'person-2': 'Lindsey Buckingham' }
          return { person: { id: req.id, name: names[req.id] } }
        },
      })
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('tab', { name: 'Members' }))

    expect(await screen.findByText('Current members')).toBeInTheDocument()
    expect(screen.getByText('Stevie Nicks')).toBeInTheDocument()
    expect(screen.getByText('vocalist (since 1975)')).toBeInTheDocument()

    expect(screen.getByText('Former members')).toBeInTheDocument()
    expect(screen.getByText('Lindsey Buckingham')).toBeInTheDocument()
    expect(screen.getByText('guitarist (1975–1987)')).toBeInTheDocument()
  })

  it('treats a person with any still-open role on this entry as current, even if another role of theirs already ended', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      router.service(EntryPersonService, {
        listEntryPeople: () => ({
          entryPeople: [
            {
              libraryEntryId: 'artist-1',
              personId: 'person-1',
              role: 'guitarist',
              startDate: timestampFromDate(new Date('1968-01-01')),
              endDate: timestampFromDate(new Date('1980-01-01')),
            },
            { libraryEntryId: 'artist-1', personId: 'person-1', role: 'producer' },
          ],
        }),
      })
      router.service(PersonService, { getPerson: () => ({ person: { id: 'person-1', name: 'Peter Green' } }) })
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('tab', { name: 'Members' }))

    expect(await screen.findByText('Current members')).toBeInTheDocument()
    expect(screen.getByText('Peter Green')).toBeInTheDocument()
    expect(screen.getByText('guitarist (1968–1980)')).toBeInTheDocument()
    expect(screen.getByText('producer')).toBeInTheDocument()
    expect(screen.queryByText('Former members')).not.toBeInTheDocument()
  })
})

describe('ArtistDetail — Edit action / Refresh from MusicBrainz', () => {
  it('disables the primary Edit segment (no manual editor yet, #681) and the menu item when the artist has no known mbid', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, { getLibraryEntry: () => ({ libraryEntry: baseEntry }) })
      registerNoProviderData(router)
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'More edit actions' }))
    expect(screen.getByRole('menuitem', { name: 'Refresh from MusicBrainz' })).toBeDisabled()
  })

  it('opens the diff dialog and refetches the artist once the update is confirmed', async () => {
    let name = 'Fleetwood Mac'
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        getLibraryEntry: () => ({ libraryEntry: { ...baseEntry, name } }),
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['name'])
          name = req.libraryEntry!.name!
          return { libraryEntry: { ...baseEntry, name } }
        },
      })
      router.service(ExternalIDService, { getExternalID: () => ({ externalId: { value: 'mbid-1' } }) })
      router.service(FanartTVService, {
        lookupArtist: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
      router.service(TheAudioDBService, {
        lookupArtist: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
      router.service(MusicBrainzService, {
        getArtist: () => ({
          artist: {
            mbid: 'mbid-1',
            name: 'Fleetwood Mac (reunion)',
            sortName: 'Fleetwood Mac',
            type: 'Group',
            country: 'GB',
            lifeSpanBegin: '1967',
            aliases: ['FM'],
          },
          isnis: ['0000000123456789'],
          officialUrl: 'https://fleetwoodmac.com/',
          wikipediaUrl: 'https://en.wikipedia.org/wiki/Fleetwood_Mac',
        }),
      })
      registerNoImages(router)
      registerNoAlbums(router)
      registerNoMembers(router)
    })

    renderArtistDetail(mockTransport)
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'More edit actions' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Refresh from MusicBrainz' }))

    await waitFor(() => expect(screen.getByText('Fleetwood Mac (reunion)')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Fleetwood Mac (reunion)' })).toBeInTheDocument())
  })
})
