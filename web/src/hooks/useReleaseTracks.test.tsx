import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import { MusicReleaseService } from '../gen/purser/music/v1/release_pb'
import { itemStatusFromProto, useReleaseTracks } from './useReleaseTracks'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend.
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

describe('useReleaseTracks', () => {
  it("lists an edition's tracks by release id", async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        listMusicReleaseTracks: req => {
          expect(req.releaseId).toBe('rel-1')
          return { tracks: [{ id: 'item-1', title: 'Track One' }, { id: 'item-2', title: 'Track Two' }] }
        },
      })
    })

    const { result } = renderHook(() => useReleaseTracks('rel-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.tracks).toHaveLength(2)
  })

  it('skips the call when releaseId is empty, e.g. before any edition is loaded', () => {
    const mockTransport = createRouterTransport(() => {})

    const { result } = renderHook(() => useReleaseTracks(''), { wrapper: wrapper(mockTransport) })

    expect(result.current.isPending).toBe(true)
    expect(result.current.tracks).toEqual([])
  })

  it('re-queries when the selected release id changes', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        listMusicReleaseTracks: req => ({
          tracks: req.releaseId === 'rel-1' ? [{ id: 'item-1', title: 'Track One' }] : [],
        }),
      })
    })

    const { result, rerender } = renderHook(({ releaseId }) => useReleaseTracks(releaseId), {
      wrapper: wrapper(mockTransport),
      initialProps: { releaseId: 'rel-1' },
    })

    await waitFor(() => expect(result.current.tracks).toHaveLength(1))

    rerender({ releaseId: 'rel-2' })

    await waitFor(() => expect(result.current.tracks).toHaveLength(0))
  })

  it('exposes refetch so track-mutating actions can refresh the list', async () => {
    let callCount = 0
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        listMusicReleaseTracks: () => {
          callCount += 1
          return { tracks: callCount === 1 ? [] : [{ id: 'item-1', title: 'Track One' }] }
        },
      })
    })

    const { result } = renderHook(() => useReleaseTracks('rel-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.tracks).toHaveLength(0))

    await result.current.refetch()

    await waitFor(() => expect(result.current.tracks).toHaveLength(1))
  })
})

describe('itemStatusFromProto', () => {
  it.each([
    [ItemStatus.WANTED, 'wanted'],
    [ItemStatus.GRABBED, 'grabbed'],
    [ItemStatus.DOWNLOADING, 'downloading'],
    [ItemStatus.IMPORTED, 'imported'],
    [ItemStatus.MISSING, 'missing'],
    [ItemStatus.SKIPPED, 'skipped'],
  ] as const)('maps proto status %s to %s', (proto, plain) => {
    expect(itemStatusFromProto(proto)).toBe(plain)
  })

  it('maps the unspecified/required-oneof default to undefined', () => {
    expect(itemStatusFromProto(ItemStatus.UNSPECIFIED)).toBeUndefined()
  })
})
