import { createRouterTransport, ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { EntityType, MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { Gender, PersonService } from '../gen/purser/domain/v1/person_pb'
import { useGetOrCreatePersonFromMusicBrainz } from './useGetOrCreatePersonFromMusicBrainz'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern useAddArtist.test.tsx
// (this hook's extraction source) already uses.
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

describe('useGetOrCreatePersonFromMusicBrainz', () => {
  it('step 1 hit: returns the existing Person id and never calls CreatePerson', async () => {
    let createCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: req => {
          expect(req.entityType).toBe(EntityType.PERSON)
          expect(req.source).toBe('mbz')
          expect(req.value).toBe('mbid-1')
          return { externalId: { entityId: 'existing-person-1' } }
        },
      })
      router.service(PersonService, {
        createPerson: () => {
          createCalled = true
          return { person: { id: 'should-not-happen' } }
        },
      })
    })

    const { result } = renderHook(() => useGetOrCreatePersonFromMusicBrainz(), { wrapper: wrapper(mockTransport) })

    let personId: string | undefined
    await act(async () => {
      personId = await result.current.getOrCreatePerson({ mbid: 'mbid-1', name: 'Stevie Nicks' })
    })

    expect(personId).toBe('existing-person-1')
    expect(createCalled).toBe(false)
  })

  it('step 2/3 win: creates the Person with the mapped fields and links it', async () => {
    const birthDate = timestampFromDate(new Date('1948-05-26'))
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(PersonService, {
        createPerson: req => {
          expect(req.person?.name).toBe('Stevie Nicks')
          expect(req.person?.sortName).toBe('Nicks, Stevie')
          expect(req.person?.gender).toBe(Gender.UNKNOWN)
          expect(req.person?.nationality).toBe('US')
          expect(req.person?.birthDate).toEqual(birthDate)
          // MonitorMode is a required oneof on domain.Person.Validate —
          // MONITOR_MODE_UNSPECIFIED (the zero value) fails it.
          expect(req.person?.monitored).toBe(true)
          expect(req.person?.monitorMode).toBe(MonitorMode.ALL)
          return { person: { id: 'person-nicks', name: req.person!.name } }
        },
      })
    })

    const { result } = renderHook(() => useGetOrCreatePersonFromMusicBrainz(), { wrapper: wrapper(mockTransport) })

    let personId: string | undefined
    await act(async () => {
      personId = await result.current.getOrCreatePerson({
        mbid: 'mbid-1',
        name: 'Stevie Nicks',
        sortName: 'Nicks, Stevie',
        nationality: 'US',
        birthDate,
      })
    })

    expect(personId).toBe('person-nicks')
  })

  it('sortName defaults to name when omitted, same as the band-member call site', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(PersonService, {
        createPerson: req => {
          expect(req.person?.sortName).toBe('Mick Fleetwood')
          expect(req.person?.nationality).toBe('')
          return { person: { id: 'person-fleetwood', name: req.person!.name } }
        },
      })
    })

    const { result } = renderHook(() => useGetOrCreatePersonFromMusicBrainz(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await result.current.getOrCreatePerson({ mbid: 'mbid-2', name: 'Mick Fleetwood' })
    })
  })

  it('step 3 loss: returns the winner id instead of the speculative create', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: () => ({ externalId: { entityId: 'winner-1' } }),
      })
      router.service(PersonService, {
        createPerson: () => ({ person: { id: 'loser-1', name: 'Stevie Nicks' } }),
      })
    })

    const { result } = renderHook(() => useGetOrCreatePersonFromMusicBrainz(), { wrapper: wrapper(mockTransport) })

    let personId: string | undefined
    await act(async () => {
      personId = await result.current.getOrCreatePerson({ mbid: 'mbid-1', name: 'Stevie Nicks' })
    })

    expect(personId).toBe('winner-1')
  })
})
