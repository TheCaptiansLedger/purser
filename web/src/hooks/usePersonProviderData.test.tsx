import { ConnectError, Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { StashDBService } from '../gen/purser/afterdark/v1/stashdb_pb'
import { ThePornDBService } from '../gen/purser/afterdark/v1/theporndb_pb'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { FanartTVService } from '../gen/purser/music/v1/fanarttv_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { TheAudioDBService } from '../gen/purser/music/v1/theaudiodb_pb'
import { WikidataService } from '../gen/purser/music/v1/wikidata_pb'
import { usePersonProviderData } from './usePersonProviderData'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// useArtistProviderData.test.tsx.
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

function notFound() {
  return () => {
    throw new ConnectError('not found', Code.NotFound)
  }
}

describe('usePersonProviderData — Music-origin, already linked (mbz ExternalID)', () => {
  it('uses the linked mbid directly and never calls MusicBrainz search', async () => {
    let searchCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalID: req => (req.source === 'mbz' ? { externalId: { value: 'mbid-1' } } : notFound()()),
      })
      router.service(FanartTVService, {
        lookupArtist: req => {
          expect(req.mbid).toBe('mbid-1')
          return { artist: { artistThumb: [{ id: '1', url: 'https://fanart/thumb.jpg', likes: '1', lang: '' }] } }
        },
      })
      router.service(TheAudioDBService, {
        lookupArtist: () => ({ artist: { thumb: 'https://audiodb/thumb.jpg' } }),
      })
      router.service(MusicBrainzService, {
        getArtist: () => ({ artist: {}, wikidataUrl: '' }),
        searchArtists: () => {
          searchCalled = true
          return { artists: [] }
        },
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', 'Jon Lord'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.mbid).toBe('mbid-1'))
    await waitFor(() =>
      expect(result.current.photoCandidates).toEqual(
        expect.arrayContaining([{ url: 'https://fanart/thumb.jpg', source: 'fanart.tv', label: 'Thumb' }]),
      ),
    )
    expect(searchCalled).toBe(false)
  })
})

describe('usePersonProviderData — Music-origin, no link, search by name', () => {
  it('searches MusicBrainz by name and chains its top match to fanart.tv/TheAudioDB/Wikidata', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, { getExternalID: notFound() })
      router.service(MusicBrainzService, {
        searchArtists: req => {
          expect(req.query).toBe('Jon Lord')
          // "Jon Lord Trio" is a real, different act that a substring
          // search could also surface — must be filtered out, only the
          // exact-name match's mbid is used.
          return { artists: [{ mbid: 'mbid-searched', name: 'Jon Lord' }, { mbid: 'mbid-second', name: 'Jon Lord Trio' }] }
        },
        getArtist: req => {
          expect(req.mbid).toBe('mbid-searched')
          return { artist: {}, wikidataUrl: 'https://www.wikidata.org/wiki/Q1' }
        },
      })
      router.service(FanartTVService, {
        lookupArtist: req => {
          expect(req.mbid).toBe('mbid-searched')
          return { artist: { artistThumb: [{ id: '1', url: 'https://fanart/thumb.jpg', likes: '1', lang: '' }] } }
        },
      })
      router.service(TheAudioDBService, { lookupArtist: () => ({ artist: {} }) })
      router.service(WikidataService, {
        lookupImage: () => ({ images: [{ url: 'https://commons/photo.jpg' }] }),
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', 'Jon Lord'), { wrapper: wrapper(mockTransport) })

    await waitFor(() =>
      expect(result.current.photoCandidates).toEqual(
        expect.arrayContaining([
          { url: 'https://fanart/thumb.jpg', source: 'fanart.tv', label: 'Thumb' },
          { url: 'https://commons/photo.jpg', source: 'wikidata', label: 'Photo' },
        ]),
      ),
    )
  })

  it('makes no MusicBrainz search call with no name and no mbz ExternalID', async () => {
    let searchCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, { getExternalID: notFound() })
      router.service(MusicBrainzService, {
        searchArtists: () => {
          searchCalled = true
          return { artists: [] }
        },
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', ''), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.mbid).toBeUndefined())
    expect(result.current.photoCandidates).toEqual([])
    expect(searchCalled).toBe(false)
  })
})

describe('usePersonProviderData — AfterDark-origin, already linked', () => {
  it('uses the linked StashDB id directly and never calls SearchPerformers', async () => {
    let searchCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalID: req => (req.source === 'stashdb' ? { externalId: { value: 'performer-1' } } : notFound()()),
      })
      router.service(StashDBService, {
        lookupPerformer: req => {
          expect(req.id).toBe('performer-1')
          return { performer: { id: 'performer-1', name: 'Jane Doe', images: [{ id: 'img-1', url: 'https://stashdb/photo.jpg' }] } }
        },
        searchPerformers: () => {
          searchCalled = true
          return { performers: [] }
        },
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', 'Jane Doe'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.stashDBId).toBe('performer-1'))
    await waitFor(() =>
      expect(result.current.photoCandidates).toEqual([{ url: 'https://stashdb/photo.jpg', source: 'stashdb', label: 'Jane Doe' }]),
    )
    expect(searchCalled).toBe(false)
  })

  it('uses the linked ThePornDB id directly and never calls SearchPerformers', async () => {
    let searchCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalID: req => (req.source === 'tpdb' ? { externalId: { value: 'performer-2' } } : notFound()()),
      })
      router.service(ThePornDBService, {
        lookupPerformer: req => {
          expect(req.id).toBe('performer-2')
          return { performer: { id: 'performer-2', name: 'Jane Doe', thumbnail: 'https://tpdb/thumb.jpg' } }
        },
        searchPerformers: () => {
          searchCalled = true
          return { performers: [] }
        },
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', 'Jane Doe'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.tpdbId).toBe('performer-2'))
    await waitFor(() =>
      expect(result.current.photoCandidates).toEqual([{ url: 'https://tpdb/thumb.jpg', source: 'theporndb', label: 'Jane Doe' }]),
    )
    expect(searchCalled).toBe(false)
  })
})

describe('usePersonProviderData — AfterDark-origin, no link, search by name', () => {
  it('searches StashDB by name, keeps only exact name/alias matches, and offers every match\'s images', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, { getExternalID: notFound() })
      router.service(StashDBService, {
        searchPerformers: req => {
          expect(req.term).toBe('Alex Coal')
          return {
            performers: [
              // Two distinct real performers can genuinely share an exact
              // name — both kept.
              { id: 'performer-1', name: 'Alex Coal', images: [{ id: 'img-1', url: 'https://stashdb/a.jpg' }] },
              { id: 'performer-2', name: 'Someone Else', aliases: ['Alex Coal'], images: [{ id: 'img-2', url: 'https://stashdb/b.jpg' }] },
              // StashDB's own free-text search is fuzzy (confirmed live) —
              // a bare substring match like this must be filtered out.
              { id: 'performer-3', name: 'Alex', images: [{ id: 'img-3', url: 'https://stashdb/c.jpg' }] },
            ],
          }
        },
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', 'Alex Coal'), { wrapper: wrapper(mockTransport) })

    await waitFor(() =>
      expect(result.current.photoCandidates).toEqual([
        { url: 'https://stashdb/a.jpg', source: 'stashdb', label: 'Alex Coal' },
        { url: 'https://stashdb/b.jpg', source: 'stashdb', label: 'Someone Else' },
      ]),
    )
  })

  it('searches ThePornDB by name and offers image/thumbnail/face plus every poster', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, { getExternalID: notFound() })
      router.service(ThePornDBService, {
        searchPerformers: req => {
          expect(req.term).toBe('Alex Coal')
          return {
            performers: [
              {
                id: 'performer-1',
                name: 'Alex Coal',
                image: 'https://tpdb/full.jpg',
                thumbnail: 'https://tpdb/thumb.jpg',
                face: 'https://tpdb/face.jpg',
                posters: [{ id: 1, url: 'https://tpdb/poster.jpg' }],
              },
            ],
          }
        },
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', 'Alex Coal'), { wrapper: wrapper(mockTransport) })

    await waitFor(() =>
      expect(result.current.photoCandidates).toEqual([
        { url: 'https://tpdb/full.jpg', source: 'theporndb', label: 'Alex Coal' },
        { url: 'https://tpdb/thumb.jpg', source: 'theporndb', label: 'Alex Coal' },
        { url: 'https://tpdb/face.jpg', source: 'theporndb', label: 'Alex Coal' },
        { url: 'https://tpdb/poster.jpg', source: 'theporndb', label: 'Alex Coal' },
      ]),
    )
  })

  it('excludes every fuzzy/unrelated StashDB search hit that does not actually share the name', async () => {
    // Regression: StashDB's own free-text search returned unrelated
    // performers for a totally unrelated name during manual testing — a
    // Music person's page must never surface those.
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, { getExternalID: notFound() })
      router.service(StashDBService, {
        searchPerformers: () => ({
          performers: [
            { id: 'performer-1', name: 'Some Other Performer', images: [{ id: 'img-1', url: 'https://stashdb/unrelated.jpg' }] },
          ],
        }),
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', 'David Coverdale'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.mbid).toBeUndefined())
    expect(result.current.photoCandidates).toEqual([])
  })

  it('degrades gracefully with no ExternalID row and no name — no provider calls made', async () => {
    let stashDBCalled = false
    let tpdbCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, { getExternalID: notFound() })
      router.service(StashDBService, {
        searchPerformers: () => {
          stashDBCalled = true
          return { performers: [] }
        },
      })
      router.service(ThePornDBService, {
        searchPerformers: () => {
          tpdbCalled = true
          return { performers: [] }
        },
      })
    })

    const { result } = renderHook(() => usePersonProviderData('person-1', ''), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.mbid).toBeUndefined())
    expect(result.current.stashDBId).toBeUndefined()
    expect(result.current.tpdbId).toBeUndefined()
    expect(result.current.photoCandidates).toEqual([])
    expect(stashDBCalled).toBe(false)
    expect(tpdbCalled).toBe(false)
  })
})
