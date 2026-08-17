import { ConnectError, Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'
import { EntityType } from '../gen/purser/domain/v1/common_pb'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { FanartTVService } from '../gen/purser/music/v1/fanarttv_pb'
import { TheAudioDBService } from '../gen/purser/music/v1/theaudiodb_pb'
import { useArtistProviderData } from './useArtistProviderData'

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

describe('useArtistProviderData', () => {
  it('resolves the mbid, then fans out to fanart.tv and TheAudioDB', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalID: req => {
          expect(req.entityType).toBe(EntityType.LIBRARY_ENTRY)
          expect(req.entityId).toBe('artist-1')
          expect(req.source).toBe('mbz')
          return { externalId: { value: 'mbid-1' } }
        },
      })
      router.service(FanartTVService, {
        lookupArtist: req => {
          expect(req.mbid).toBe('mbid-1')
          return {
            artist: {
              artistBackground: [{ id: '1', url: 'https://fanart/bg.jpg', likes: '1', lang: '' }],
              hdMusicLogo: [{ id: '2', url: 'https://fanart/logo.png', likes: '1', lang: '' }],
              artistThumb: [{ id: '3', url: 'https://fanart/thumb.jpg', likes: '1', lang: '' }],
            },
          }
        },
      })
      router.service(TheAudioDBService, {
        lookupArtist: req => {
          expect(req.mbid).toBe('mbid-1')
          return { artist: { biography: 'A band.' } }
        },
      })
    })

    const { result } = renderHook(() => useArtistProviderData('artist-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.mbid).toBe('mbid-1'))
    await waitFor(() => expect(result.current.backdropUrl).toBe('https://fanart/bg.jpg'))
    expect(result.current.bio).toBe('A band.')
    expect(result.current.backdropCandidates).toEqual([
      { url: 'https://fanart/bg.jpg', source: 'fanart.tv', label: 'Background' },
      { url: 'https://fanart/logo.png', source: 'fanart.tv', label: 'Logo' },
    ])
    expect(result.current.posterCandidates).toEqual([
      { url: 'https://fanart/thumb.jpg', source: 'fanart.tv', label: 'Thumb' },
    ])
  })

  it('degrades gracefully with no mbz ExternalID row — no provider calls made', async () => {
    let fanartCalled = false
    let audiodbCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalID: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
      })
      router.service(FanartTVService, {
        lookupArtist: () => {
          fanartCalled = true
          return { artist: undefined }
        },
      })
      router.service(TheAudioDBService, {
        lookupArtist: () => {
          audiodbCalled = true
          return { artist: undefined }
        },
      })
    })

    const { result } = renderHook(() => useArtistProviderData('artist-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.mbid).toBeUndefined())
    expect(result.current.backdropUrl).toBeUndefined()
    expect(result.current.bio).toBeUndefined()
    expect(result.current.backdropCandidates).toEqual([])
    expect(result.current.posterCandidates).toEqual([])
    expect(fanartCalled).toBe(false)
    expect(audiodbCalled).toBe(false)
  })

  it('degrades gracefully when a provider 404s despite a valid mbid', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ExternalIDService, {
        getExternalID: () => ({ externalId: { value: 'mbid-1' } }),
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
    })

    const { result } = renderHook(() => useArtistProviderData('artist-1'), { wrapper: wrapper(mockTransport) })

    await waitFor(() => expect(result.current.mbid).toBe('mbid-1'))
    expect(result.current.backdropUrl).toBeUndefined()
    expect(result.current.bio).toBeUndefined()
    expect(result.current.backdropCandidates).toEqual([])
    expect(result.current.posterCandidates).toEqual([])
  })
})
