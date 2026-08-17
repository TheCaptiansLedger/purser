import { createRouterTransport, ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { TheAudioDBService } from '../gen/purser/music/v1/theaudiodb_pb'
import type { MusicBrainzArtist } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { libraryEntryMetadataFromMusicBrainzArtist, useAddArtist } from './useAddArtist'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as useAttachImage.test.tsx,
// the closest existing analog to this hook's multi-call composition.
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

const groupCandidate: MusicBrainzArtist = {
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
}

// tadbMiss registers a NotFound-throwing LookupArtist handler — the
// common case in these tests, isolating the get-or-create composition
// from the best-effort enrichment step.
function tadbMiss(router: Parameters<Parameters<typeof createRouterTransport>[0]>[0]) {
  router.service(TheAudioDBService, {
    lookupArtist: () => {
      throw new ConnectError('not found', Code.NotFound)
    },
  })
}

describe('libraryEntryMetadataFromMusicBrainzArtist', () => {
  it('maps a Group to founded_date/dissolved_date, artist_type, and aliases', () => {
    expect(libraryEntryMetadataFromMusicBrainzArtist(groupCandidate)).toEqual({
      artist_type: 'Group',
      aliases: ['REO'],
      founded_date: '1967',
    })
  })

  it('maps a Person to born_date/died_date', () => {
    const person: MusicBrainzArtist = { ...groupCandidate, type: 'Person', lifeSpanBegin: '1940', lifeSpanEnd: '2016', aliases: [] }
    expect(libraryEntryMetadataFromMusicBrainzArtist(person)).toEqual({
      artist_type: 'Person',
      born_date: '1940',
      died_date: '2016',
    })
  })

  it('omits empty fields rather than writing empty strings/arrays', () => {
    const bare: MusicBrainzArtist = { ...groupCandidate, type: '', lifeSpanBegin: '', lifeSpanEnd: '', aliases: [] }
    expect(libraryEntryMetadataFromMusicBrainzArtist(bare)).toEqual({})
  })
})

describe('useAddArtist', () => {
  it('step 1 hit: returns the existing artist and never calls CreateLibraryEntry', async () => {
    let createCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: req => {
          expect(req.value).toBe('mbid-1')
          expect(req.source).toBe('mbz')
          return { externalId: { entityId: 'existing-1' } }
        },
      })
      router.service(LibraryEntryService, {
        getLibraryEntry: req => {
          expect(req.id).toBe('existing-1')
          return { libraryEntry: { id: 'existing-1', name: 'REO Speedwagon' } }
        },
        createLibraryEntry: () => {
          createCalled = true
          return { libraryEntry: { id: 'should-not-happen' } }
        },
      })
      tadbMiss(router)
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({ id: 'existing-1' })
    expect(createCalled).toBe(false)
  })

  it('step 2/3 win: creates the entry and links it, no delete', async () => {
    let deleteCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => {
          expect(req.externalId?.entityId).toBe('new-1')
          return { externalId: { entityId: 'new-1' } }
        },
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: req => {
          expect(req.libraryEntry?.name).toBe('REO Speedwagon')
          expect(req.libraryEntry?.metadata).toEqual({ artist_type: 'Group', aliases: ['REO'], founded_date: '1967' })
          // Regression: MonitorMode is a required oneof on
          // domain.LibraryEntry.Validate — MONITOR_MODE_UNSPECIFIED (the
          // zero value) fails it with "MonitorMode: failed required".
          expect(req.libraryEntry?.monitored).toBe(true)
          expect(req.libraryEntry?.monitorMode).toBe(MonitorMode.ALL)
          return { libraryEntry: { id: 'new-1', name: 'REO Speedwagon' } }
        },
        deleteLibraryEntry: () => {
          deleteCalled = true
          return {}
        },
      })
      tadbMiss(router)
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({ id: 'new-1' })
    expect(deleteCalled).toBe(false)
  })

  it('step 3 loss: deletes its speculative entry and returns the winner', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: () => ({ externalId: { entityId: 'winner-1' } }),
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => ({ libraryEntry: { id: 'loser-1', name: 'REO Speedwagon' } }),
        deleteLibraryEntry: req => {
          expect(req.id).toBe('loser-1')
          expect(req.cascade).toBe(false)
          return {}
        },
        getLibraryEntry: req => {
          expect(req.id).toBe('winner-1')
          return { libraryEntry: { id: 'winner-1', name: 'REO Speedwagon' } }
        },
      })
      tadbMiss(router)
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({ id: 'winner-1' })
  })

  it('TheAudioDB hit: merges genre/style into Metadata via a field-masked update', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => ({
          libraryEntry: { id: 'new-1', name: 'REO Speedwagon', metadata: { artist_type: 'Group' } },
        }),
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['metadata'])
          expect(req.libraryEntry?.metadata).toEqual({ artist_type: 'Group', genre: 'Rock', style: 'Arena Rock' })
          return { libraryEntry: { id: 'new-1', name: 'REO Speedwagon', metadata: req.libraryEntry!.metadata } }
        },
      })
      router.service(TheAudioDBService, {
        lookupArtist: () => ({ artist: { genre: 'Rock', style: 'Arena Rock' } }),
      })
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({ metadata: { genre: 'Rock', style: 'Arena Rock' } })
  })

  it('TheAudioDB miss: never calls UpdateLibraryEntry and still succeeds', async () => {
    let updateCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => ({ libraryEntry: { id: 'new-1', name: 'REO Speedwagon' } }),
        updateLibraryEntry: () => {
          updateCalled = true
          return { libraryEntry: {} }
        },
      })
      tadbMiss(router)
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({ id: 'new-1' })
    expect(updateCalled).toBe(false)
  })

  // Acceptance criterion: two callers importing the same MBID at once
  // resolve to exactly one surviving LibraryEntry. The mock transport
  // plays the server's own reservation-document behavior (ADR 0026):
  // both speculative creates succeed with distinct ids, but
  // CreateExternalID always resolves to whichever one arrived first —
  // the second caller's Create call — since Create itself is the
  // reservation write. First caller's own composition should then see
  // its own id echoed back (win); the second caller's should see the
  // first caller's id (loss) and clean up.
  it('concurrent duplicate add: exactly one LibraryEntry survives for the same MBID', async () => {
    let created = 0
    let winningEntityId: string | undefined
    const deletedIds: string[] = []

    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => {
          // First CreateExternalID call to land wins the reservation —
          // every subsequent call for the same value is told the same
          // winner, exactly ExternalIDRepository.Create's get-or-create
          // contract (ADR 0026).
          if (winningEntityId === undefined) {
            winningEntityId = req.externalId!.entityId
          }
          return { externalId: { entityId: winningEntityId } }
        },
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => {
          created += 1
          return { libraryEntry: { id: `caller-${created}`, name: 'REO Speedwagon' } }
        },
        deleteLibraryEntry: req => {
          deletedIds.push(req.id)
          return {}
        },
        getLibraryEntry: req => ({ libraryEntry: { id: req.id, name: 'REO Speedwagon' } }),
      })
      tadbMiss(router)
    })

    const hookA = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })
    const hookB = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entryA
    let entryB
    await act(async () => {
      ;[entryA, entryB] = await Promise.all([
        hookA.result.current.addArtist(groupCandidate),
        hookB.result.current.addArtist(groupCandidate),
      ])
    })

    expect(deletedIds).toHaveLength(1)
    expect(entryA).toMatchObject({ id: winningEntityId })
    expect(entryB).toMatchObject({ id: winningEntityId })
    expect(deletedIds[0]).not.toBe(winningEntityId)
  })
})
