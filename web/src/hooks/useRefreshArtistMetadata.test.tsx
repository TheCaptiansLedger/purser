import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import type { GetMusicBrainzArtistResponse } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { buildArtistMetadataUpdate, computeArtistMetadataDiff, useRefreshArtistMetadata } from './useRefreshArtistMetadata'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useAddArtist.test.tsx.
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

const baseEntry: LibraryEntry = {
  $typeName: 'purser.domain.v1.LibraryEntry',
  id: 'artist-1',
  contentType: 'music',
  kind: 'artist',
  name: 'REO Speedwagon',
  sortName: 'REO Speedwagon',
  overview: '',
  parentId: '',
  monitored: true,
  monitorMode: MonitorMode.ALL,
  status: '',
  qualityProfileId: '',
  metadataProfileId: '',
  path: '',
  metadata: {
    artist_type: 'Group',
    aliases: ['REO'],
    founded_date: '1967',
    isni: '0000000123456789',
    official_url: 'https://reospeedwagon.com',
    genre: 'Rock',
  },
}

const upToDateResponse: GetMusicBrainzArtistResponse = {
  $typeName: 'purser.music.v1.GetMusicBrainzArtistResponse',
  artist: {
    $typeName: 'purser.music.v1.MusicBrainzArtist',
    mbid: 'mbid-1',
    name: 'REO Speedwagon',
    sortName: 'REO Speedwagon',
    disambiguation: '',
    type: 'Group',
    country: 'US',
    lifeSpanBegin: '1967',
    lifeSpanEnd: '',
    aliases: ['REO'],
  },
  isnis: ['0000000123456789'],
  officialUrl: 'https://reospeedwagon.com',
  wikipediaUrl: '',
  wikidataUrl: '',
  members: [],
}

describe('computeArtistMetadataDiff', () => {
  it('returns no rows when every field already matches (MusicBrainz gaps included)', () => {
    expect(computeArtistMetadataDiff(baseEntry, upToDateResponse)).toEqual([])
  })

  it('returns no rows when the response has no artist', () => {
    expect(computeArtistMetadataDiff(baseEntry, { ...upToDateResponse, artist: undefined })).toEqual([])
  })

  it('flags every field that actually differs, and only those', () => {
    const response: GetMusicBrainzArtistResponse = {
      ...upToDateResponse,
      artist: {
        ...upToDateResponse.artist!,
        name: 'R.E.O. Speedwagon',
        sortName: 'Speedwagon, R.E.O.',
        aliases: ['REO', 'Speedwagon'],
        lifeSpanEnd: '1980',
      },
      isnis: ['9999999999999999'],
      wikipediaUrl: 'https://en.wikipedia.org/wiki/REO_Speedwagon',
    }

    const diff = computeArtistMetadataDiff(baseEntry, response)

    expect(diff).toEqual([
      { path: 'name', label: 'Name', current: 'REO Speedwagon', proposed: 'R.E.O. Speedwagon' },
      { path: 'sort_name', label: 'Sort name', current: 'REO Speedwagon', proposed: 'Speedwagon, R.E.O.' },
      {
        path: 'metadata',
        metadataKey: 'aliases',
        label: 'Aliases',
        current: ['REO'],
        proposed: ['REO', 'Speedwagon'],
      },
      {
        path: 'metadata',
        metadataKey: 'dissolved_date',
        label: 'Dissolved',
        current: undefined,
        proposed: '1980',
      },
      {
        path: 'metadata',
        metadataKey: 'isni',
        label: 'ISNI',
        current: '0000000123456789',
        proposed: '9999999999999999',
      },
      {
        path: 'metadata',
        metadataKey: 'wikipedia_url',
        label: 'Wikipedia',
        current: undefined,
        proposed: 'https://en.wikipedia.org/wiki/REO_Speedwagon',
      },
    ])
  })

  it('uses born/died for a solo Person artist instead of founded/dissolved', () => {
    const soloEntry: LibraryEntry = {
      ...baseEntry,
      metadata: { artist_type: 'Person', born_date: '1940' },
    }
    const response: GetMusicBrainzArtistResponse = {
      ...upToDateResponse,
      artist: { ...upToDateResponse.artist!, type: 'Person', lifeSpanBegin: '1940', lifeSpanEnd: '2016', aliases: [] },
    }

    const diff = computeArtistMetadataDiff(soloEntry, response)

    expect(diff).toContainEqual({
      path: 'metadata',
      metadataKey: 'died_date',
      label: 'Died',
      current: undefined,
      proposed: '2016',
    })
    expect(diff.some(field => field.metadataKey === 'dissolved_date' || field.metadataKey === 'founded_date')).toBe(false)
  })

  it('never proposes clearing a field MusicBrainz returns empty', () => {
    const response: GetMusicBrainzArtistResponse = {
      ...upToDateResponse,
      officialUrl: '',
      artist: { ...upToDateResponse.artist!, aliases: [] },
    }

    const diff = computeArtistMetadataDiff(baseEntry, response)

    expect(diff).toEqual([])
  })
})

describe('buildArtistMetadataUpdate', () => {
  it('masks exactly the top-level paths the diff touched, merging metadata onto the existing bag', () => {
    const diff = computeArtistMetadataDiff(baseEntry, {
      ...upToDateResponse,
      artist: { ...upToDateResponse.artist!, name: 'R.E.O. Speedwagon' },
      wikipediaUrl: 'https://en.wikipedia.org/wiki/REO_Speedwagon',
    })

    const update = buildArtistMetadataUpdate(baseEntry, diff)

    expect(update.updateMask.paths.sort()).toEqual(['metadata', 'name'])
    expect(update.libraryEntry.name).toBe('R.E.O. Speedwagon')
    expect(update.libraryEntry.sortName).toBeUndefined()
    expect(update.libraryEntry.metadata).toEqual({
      artist_type: 'Group',
      aliases: ['REO'],
      founded_date: '1967',
      isni: '0000000123456789',
      official_url: 'https://reospeedwagon.com',
      genre: 'Rock',
      wikipedia_url: 'https://en.wikipedia.org/wiki/REO_Speedwagon',
    })
  })

  it('produces an empty field-mask when there is nothing to change', () => {
    const update = buildArtistMetadataUpdate(baseEntry, [])
    expect(update.updateMask.paths).toEqual([])
    expect(update.libraryEntry).toEqual({ id: 'artist-1' })
  })
})

describe('useRefreshArtistMetadata', () => {
  it('fetches GetArtist, computes the diff, and applies exactly the changed fields', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        getArtist: req => {
          expect(req.mbid).toBe('mbid-1')
          return { ...upToDateResponse, artist: { ...upToDateResponse.artist!, name: 'R.E.O. Speedwagon' } }
        },
      })
      router.service(LibraryEntryService, {
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['name'])
          expect(req.libraryEntry?.name).toBe('R.E.O. Speedwagon')
          return { libraryEntry: { ...baseEntry, name: 'R.E.O. Speedwagon' } }
        },
      })
    })

    const { result } = renderHook(() => useRefreshArtistMetadata(baseEntry, 'mbid-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.diff).toHaveLength(1)

    let updated: LibraryEntry | undefined
    await act(async () => {
      updated = await result.current.apply()
    })

    expect(updated?.name).toBe('R.E.O. Speedwagon')
  })

  it('reports no diff rows for an already-up-to-date artist', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, { getArtist: () => upToDateResponse })
    })

    const { result } = renderHook(() => useRefreshArtistMetadata(baseEntry, 'mbid-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.diff).toEqual([])
  })
})
