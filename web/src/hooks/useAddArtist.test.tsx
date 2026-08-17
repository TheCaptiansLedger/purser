import { createRouterTransport, ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
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

// tadbMiss registers a NotFound-throwing TheAudioDB LookupArtist handler —
// the common case in these tests, isolating the get-or-create composition
// from that best-effort enrichment step.
function tadbMiss(router: Parameters<Parameters<typeof createRouterTransport>[0]>[0]) {
  router.service(TheAudioDBService, {
    lookupArtist: () => {
      throw new ConnectError('not found', Code.NotFound)
    },
  })
}

// mbzRelationsMiss registers a NotFound-throwing MusicBrainzService.GetArtist
// handler — isolates the get-or-create composition from the
// MusicBrainz-relations best-effort enrichment step (ISNI/links/band
// members), same role as tadbMiss above.
function mbzRelationsMiss(router: Parameters<Parameters<typeof createRouterTransport>[0]>[0]) {
  router.service(MusicBrainzService, {
    getArtist: () => {
      throw new ConnectError('not found', Code.NotFound)
    },
  })
}

describe('libraryEntryMetadataFromMusicBrainzArtist', () => {
  it('maps a Group to founded_date/dissolved_date, artist_type, country, and aliases', () => {
    expect(libraryEntryMetadataFromMusicBrainzArtist(groupCandidate)).toEqual({
      artist_type: 'Group',
      country: 'US',
      aliases: ['REO'],
      founded_date: '1967',
    })
  })

  it('maps a Person to born_date/died_date', () => {
    const person: MusicBrainzArtist = { ...groupCandidate, type: 'Person', lifeSpanBegin: '1940', lifeSpanEnd: '2016', aliases: [] }
    expect(libraryEntryMetadataFromMusicBrainzArtist(person)).toEqual({
      artist_type: 'Person',
      country: 'US',
      born_date: '1940',
      died_date: '2016',
    })
  })

  it('omits empty fields rather than writing empty strings/arrays', () => {
    const bare: MusicBrainzArtist = { ...groupCandidate, type: '', country: '', lifeSpanBegin: '', lifeSpanEnd: '', aliases: [] }
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
          expect(req.libraryEntry?.metadata).toEqual({ artist_type: 'Group', country: 'US', aliases: ['REO'], founded_date: '1967' })
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
      mbzRelationsMiss(router)
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
      mbzRelationsMiss(router)
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
      mbzRelationsMiss(router)
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
      mbzRelationsMiss(router)
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({ id: 'new-1' })
    expect(updateCalled).toBe(false)
  })

  it('MusicBrainz-relations hit: merges isni/official_url/wikipedia_url into Metadata', async () => {
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
          expect(req.libraryEntry?.metadata).toEqual({
            artist_type: 'Group',
            isni: '0000000123456789',
            official_url: 'http://www.speedwagon.com/',
            wikipedia_url: 'https://en.wikipedia.org/wiki/REO_Speedwagon',
          })
          return { libraryEntry: { id: 'new-1', name: 'REO Speedwagon', metadata: req.libraryEntry!.metadata } }
        },
      })
      tadbMiss(router)
      router.service(MusicBrainzService, {
        getArtist: () => ({
          artist: { mbid: 'mbid-1', name: 'REO Speedwagon' },
          isnis: ['0000000123456789'],
          officialUrl: 'http://www.speedwagon.com/',
          wikipediaUrl: 'https://en.wikipedia.org/wiki/REO_Speedwagon',
          members: [],
        }),
      })
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({
      metadata: { isni: '0000000123456789', official_url: 'http://www.speedwagon.com/', wikipedia_url: 'https://en.wikipedia.org/wiki/REO_Speedwagon' },
    })
  })

  it('MusicBrainz-relations miss: never calls UpdateLibraryEntry and still succeeds', async () => {
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
      mbzRelationsMiss(router)
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    let entry
    await act(async () => {
      entry = await result.current.addArtist(groupCandidate)
    })

    expect(entry).toMatchObject({ id: 'new-1' })
    expect(updateCalled).toBe(false)
  })

  it('Group artist with band members: get-or-creates a Person per member and links each via CreateEntryPerson', async () => {
    const createdPersonNames: string[] = []
    const linkedEntryPeople: { personId: string; role: string }[] = []

    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: req => {
          // LibraryEntry lookup misses; the two PERSON lookups (Cronin,
          // Doughty) also miss — every member is a fresh Person here.
          expect(['mbid-1', 'member-cronin', 'member-doughty']).toContain(req.value)
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => ({ libraryEntry: { id: 'artist-1', name: 'REO Speedwagon' } }),
      })
      router.service(PersonService, {
        createPerson: req => {
          createdPersonNames.push(req.person!.name)
          // Regression: MonitorMode is a required oneof on
          // domain.Person.Validate — MONITOR_MODE_UNSPECIFIED (the zero
          // value) fails it server-side with "MonitorMode: failed
          // required". A mock that echoes back whatever's sent, like this
          // one, can't catch a missing field on its own — this assertion
          // is what actually catches it.
          expect(req.person?.monitored).toBe(true)
          expect(req.person?.monitorMode).toBe(MonitorMode.ALL)
          return { person: { id: `person-${req.person!.name}`, name: req.person!.name } }
        },
      })
      router.service(EntryPersonService, {
        createEntryPerson: req => {
          linkedEntryPeople.push({ personId: req.entryPerson!.personId, role: req.entryPerson!.role })
          return { entryPerson: req.entryPerson }
        },
      })
      tadbMiss(router)
      router.service(MusicBrainzService, {
        getArtist: () => ({
          artist: { mbid: 'mbid-1', name: 'REO Speedwagon' },
          isnis: [],
          officialUrl: '',
          wikipediaUrl: '',
          members: [
            { mbid: 'member-cronin', name: 'Kevin Cronin', attributes: ['vocal', 'guitar'], begin: '1972', end: '', ended: false },
            { mbid: 'member-doughty', name: 'Neal Doughty', attributes: [], begin: '1967', end: '', ended: false },
          ],
        }),
      })
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await result.current.addArtist(groupCandidate)
    })

    expect(createdPersonNames.sort()).toEqual(['Kevin Cronin', 'Neal Doughty'])
    expect(linkedEntryPeople).toContainEqual({ personId: 'person-Kevin Cronin', role: 'vocal, guitar' })
    // No attributes on the relation falls back to a non-empty Role —
    // domain.EntryPerson.Validate requires one.
    expect(linkedEntryPeople).toContainEqual({ personId: 'person-Neal Doughty', role: 'Member' })
  })

  it('Group artist, member already linked as a Person: reuses the existing Person, never re-creates it', async () => {
    let createPersonCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: req => {
          if (req.value === 'member-cronin') return { externalId: { entityId: 'existing-person-1' } }
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => ({ libraryEntry: { id: 'artist-1', name: 'REO Speedwagon' } }),
      })
      router.service(PersonService, {
        createPerson: () => {
          createPersonCalled = true
          return { person: { id: 'should-not-happen' } }
        },
      })
      router.service(EntryPersonService, {
        createEntryPerson: req => ({ entryPerson: req.entryPerson }),
      })
      tadbMiss(router)
      router.service(MusicBrainzService, {
        getArtist: () => ({
          artist: { mbid: 'mbid-1', name: 'REO Speedwagon' },
          isnis: [],
          officialUrl: '',
          wikipediaUrl: '',
          members: [{ mbid: 'member-cronin', name: 'Kevin Cronin', attributes: [], begin: '', end: '', ended: false }],
        }),
      })
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await result.current.addArtist(groupCandidate)
    })

    expect(createPersonCalled).toBe(false)
  })

  it('Person (solo) artist with relations data present: never creates band-member Person/EntryPerson rows', async () => {
    let createPersonCalled = false
    let createEntryPersonCalled = false
    const soloCandidate: MusicBrainzArtist = { ...groupCandidate, type: 'Person' }

    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => ({ libraryEntry: { id: 'artist-1', name: 'Stevie Nicks' } }),
      })
      router.service(PersonService, {
        createPerson: () => {
          createPersonCalled = true
          return { person: { id: 'should-not-happen' } }
        },
      })
      router.service(EntryPersonService, {
        createEntryPerson: () => {
          createEntryPersonCalled = true
          return { entryPerson: {} }
        },
      })
      tadbMiss(router)
      router.service(MusicBrainzService, {
        // A MusicBrainz "member of band" edge would never actually appear
        // on a solo Person's own relations, but if one somehow did, a
        // Type=="Person" candidate must still never trigger member
        // creation — that's #672's own human-driven linking step.
        getArtist: () => ({
          artist: { mbid: 'mbid-1', name: 'Stevie Nicks' },
          isnis: [],
          officialUrl: '',
          wikipediaUrl: '',
          members: [{ mbid: 'member-x', name: 'Someone', attributes: [], begin: '', end: '', ended: false }],
        }),
      })
    })

    const { result } = renderHook(() => useAddArtist(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await result.current.addArtist(soloCandidate)
    })

    expect(createPersonCalled).toBe(false)
    expect(createEntryPersonCalled).toBe(false)
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
      mbzRelationsMiss(router)
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
