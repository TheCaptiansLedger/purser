import { createRouterTransport, ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import type { MusicBrainzReleaseGroup } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { albumType, useAddAlbum } from './useAddAlbum'

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

const releaseGroupCandidate: MusicBrainzReleaseGroup = {
  $typeName: 'purser.music.v1.MusicBrainzReleaseGroup',
  mbid: 'rg-mbid-1',
  title: 'Hi Infidelity',
  disambiguation: '',
  primaryType: 'Album',
  secondaryTypes: [],
  firstReleaseDate: '1980-11-21',
}

const artistId = 'artist-1'

describe('albumType', () => {
  it('maps primary type Album to studio', () => {
    expect(albumType(releaseGroupCandidate)).toBe('studio')
  })

  it('checks secondary types first: Live wins over primary type Album', () => {
    expect(albumType({ ...releaseGroupCandidate, secondaryTypes: ['Live'] })).toBe('live')
  })

  it('checks secondary types first: Compilation wins over primary type Album', () => {
    expect(albumType({ ...releaseGroupCandidate, secondaryTypes: ['Compilation'] })).toBe('compilation')
  })

  it('maps EP and Single primary types directly', () => {
    expect(albumType({ ...releaseGroupCandidate, primaryType: 'EP' })).toBe('ep')
    expect(albumType({ ...releaseGroupCandidate, primaryType: 'Single' })).toBe('single')
  })

  it('maps an unrecognized primary type to other, and an empty one to empty', () => {
    expect(albumType({ ...releaseGroupCandidate, primaryType: 'Broadcast' })).toBe('other')
    expect(albumType({ ...releaseGroupCandidate, primaryType: '' })).toBe('')
  })
})

describe('useAddAlbum', () => {
  it('step 1 hit: returns the existing album and never calls CreateGroup', async () => {
    let createCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: req => {
          expect(req.value).toBe('rg-mbid-1')
          expect(req.source).toBe('mbz')
          return { externalId: { entityId: 'existing-1' } }
        },
      })
      router.service(GroupService, {
        getGroup: req => {
          expect(req.id).toBe('existing-1')
          return { group: { id: 'existing-1', title: 'Hi Infidelity' } }
        },
        createGroup: () => {
          createCalled = true
          return { group: { id: 'should-not-happen' } }
        },
      })
    })

    const { result } = renderHook(() => useAddAlbum(), { wrapper: wrapper(mockTransport) })

    let group
    await act(async () => {
      group = await result.current.addAlbum(releaseGroupCandidate, artistId)
    })

    expect(group).toMatchObject({ id: 'existing-1' })
    expect(createCalled).toBe(false)
  })

  it('step 2/3 win: creates the group and links it, no delete', async () => {
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
      router.service(GroupService, {
        createGroup: req => {
          expect(req.group?.libraryEntryId).toBe(artistId)
          expect(req.group?.title).toBe('Hi Infidelity')
          expect(req.group?.metadata).toEqual({ album_type: 'studio' })
          // Regression: MonitorMode is a required oneof on
          // domain.Group.Validate — MONITOR_MODE_UNSPECIFIED (the zero
          // value) fails it with "MonitorMode: failed required".
          expect(req.group?.monitored).toBe(true)
          expect(req.group?.monitorMode).toBe(MonitorMode.ALL)
          return { group: { id: 'new-1', title: 'Hi Infidelity' } }
        },
        deleteGroup: () => {
          deleteCalled = true
          return {}
        },
      })
    })

    const { result } = renderHook(() => useAddAlbum(), { wrapper: wrapper(mockTransport) })

    let group
    await act(async () => {
      group = await result.current.addAlbum(releaseGroupCandidate, artistId)
    })

    expect(group).toMatchObject({ id: 'new-1' })
    expect(deleteCalled).toBe(false)
  })

  it('omits album_type from Metadata when albumType resolves empty', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(GroupService, {
        createGroup: req => {
          expect(req.group?.metadata).toEqual({})
          return { group: { id: 'new-1', title: 'Untitled' } }
        },
      })
    })

    const { result } = renderHook(() => useAddAlbum(), { wrapper: wrapper(mockTransport) })

    await act(async () => {
      await result.current.addAlbum({ ...releaseGroupCandidate, primaryType: '' }, artistId)
    })
  })

  it('step 3 loss: deletes its speculative group and returns the winner', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: () => ({ externalId: { entityId: 'winner-1' } }),
      })
      router.service(GroupService, {
        createGroup: () => ({ group: { id: 'loser-1', title: 'Hi Infidelity' } }),
        deleteGroup: req => {
          expect(req.id).toBe('loser-1')
          expect(req.cascade).toBe(false)
          return {}
        },
        getGroup: req => {
          expect(req.id).toBe('winner-1')
          return { group: { id: 'winner-1', title: 'Hi Infidelity' } }
        },
      })
    })

    const { result } = renderHook(() => useAddAlbum(), { wrapper: wrapper(mockTransport) })

    let group
    await act(async () => {
      group = await result.current.addAlbum(releaseGroupCandidate, artistId)
    })

    expect(group).toMatchObject({ id: 'winner-1' })
  })

  it('concurrent duplicate add: exactly one Group survives for the same MBID', async () => {
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
      router.service(GroupService, {
        createGroup: () => {
          created += 1
          return { group: { id: `caller-${created}`, title: 'Hi Infidelity' } }
        },
        deleteGroup: req => {
          deletedIds.push(req.id)
          return {}
        },
        getGroup: req => ({ group: { id: req.id, title: 'Hi Infidelity' } }),
      })
    })

    const hookA = renderHook(() => useAddAlbum(), { wrapper: wrapper(mockTransport) })
    const hookB = renderHook(() => useAddAlbum(), { wrapper: wrapper(mockTransport) })

    let groupA
    let groupB
    await act(async () => {
      ;[groupA, groupB] = await Promise.all([
        hookA.result.current.addAlbum(releaseGroupCandidate, artistId),
        hookB.result.current.addAlbum(releaseGroupCandidate, artistId),
      ])
    })

    expect(deletedIds).toHaveLength(1)
    expect(groupA).toMatchObject({ id: winningEntityId })
    expect(groupB).toMatchObject({ id: winningEntityId })
    expect(deletedIds[0]).not.toBe(winningEntityId)
  })
})
