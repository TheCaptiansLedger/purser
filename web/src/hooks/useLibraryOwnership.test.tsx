import { createRouterTransport } from '@connectrpc/connect'
import { ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import { MusicReleaseService } from '../gen/purser/music/v1/release_pb'
import { ReleaseStatus } from '../gen/purser/music/v1/release_pb'
import { useLibraryOwnership } from './useLibraryOwnership'

// ADR 0004: mocked transport, never a live backend — same
// createRouterTransport pattern useLibraryEntriesByIds.test.tsx uses for
// the sibling fan-out hook this one extends to two levels.
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

describe('useLibraryOwnership', () => {
  it('counts an artist\'s imported-default-edition groups as owned, out of its total groups', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        listGroups: () => ({
          groups: [
            { id: 'group-1', title: 'Rumours' },
            { id: 'group-2', title: 'Tusk' },
          ],
          nextPageToken: '',
        }),
      })
      router.service(MusicReleaseService, {
        listMusicReleases: req => {
          if (req.groupId === 'group-1') {
            return { musicReleases: [{ id: 'r1', isDefault: true, status: ReleaseStatus.IMPORTED }] }
          }
          return { musicReleases: [{ id: 'r2', isDefault: true, status: ReleaseStatus.STUB }] }
        },
      })
    })

    const { result } = renderHook(() => useLibraryOwnership(['artist-1']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current['artist-1'].isPending).toBe(false))
    expect(result.current['artist-1']).toMatchObject({ owned: 1, total: 2, isError: false })
  })

  it('reports an artist with zero groups as total 0, owned 0', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, { listGroups: () => ({ groups: [], nextPageToken: '' }) })
      router.service(MusicReleaseService, { listMusicReleases: () => ({ musicReleases: [] }) })
    })

    const { result } = renderHook(() => useLibraryOwnership(['artist-empty']), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current['artist-empty'].isPending).toBe(false))
    expect(result.current['artist-empty']).toMatchObject({ owned: 0, total: 0, isError: false })
  })

  it("isolates one artist's ListGroups failure from another artist's own result", async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        listGroups: req => {
          if (req.libraryEntryId === 'artist-bad') {
            throw new ConnectError('malformed record', Code.Internal)
          }
          return { groups: [{ id: 'group-1', title: 'Rumours' }], nextPageToken: '' }
        },
      })
      router.service(MusicReleaseService, {
        listMusicReleases: () => ({ musicReleases: [{ id: 'r1', isDefault: true, status: ReleaseStatus.IMPORTED }] }),
      })
    })

    const { result } = renderHook(() => useLibraryOwnership(['artist-bad', 'artist-good']), {
      wrapper: wrapper(mockTransport),
    })

    await waitFor(() => expect(result.current['artist-bad'].isPending).toBe(false))
    await waitFor(() => expect(result.current['artist-good'].isPending).toBe(false))
    expect(result.current['artist-bad'].isError).toBe(true)
    expect(result.current['artist-good']).toMatchObject({ owned: 1, total: 1, isError: false })
  })

  it('reports an empty map with no ids at all', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useLibraryOwnership([]), { wrapper: wrapper(mockTransport) })

    expect(result.current).toEqual({})
  })
})
