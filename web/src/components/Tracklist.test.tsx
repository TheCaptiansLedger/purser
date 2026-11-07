import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import type { Item } from '../gen/purser/domain/v1/item_pb'
import { ItemService } from '../gen/purser/domain/v1/item_pb'
import { MediaFileService } from '../gen/purser/domain/v1/media_file_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { MusicReleaseService } from '../gen/purser/music/v1/release_pb'
import { Tracklist, type TracklistProps } from './Tracklist'
import { PlayerProvider } from './PlayerProvider'
import { usePlayer } from '../hooks/usePlayer'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend. Wrapped in PlayerProvider — Tracklist dispatches into it
// (#751) — with a small state-reading consumer alongside so tests can
// assert on the resulting player state without reaching into internals.
function renderTracklist(props: Partial<TracklistProps>, mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const refetch = vi.fn()
  const fullProps: TracklistProps = {
    releaseId: 'release-1',
    mbid: '',
    tracks: [],
    isPending: false,
    refetch,
    ...props,
  }

  function PlayerStateProbe() {
    const { state } = usePlayer()
    return (
      <div data-testid="player-probe" data-current={state.current?.title ?? ''} data-queue={state.queue.map(t => t.title).join(',')} />
    )
  }

  render(
    <PlayerProvider>
      <TransportProvider transport={mockTransport}>
        <QueryClientProvider client={queryClient}>
          <Tracklist {...fullProps} />
        </QueryClientProvider>
      </TransportProvider>
      <PlayerStateProbe />
    </PlayerProvider>,
  )
  return { refetch }
}

function track(overrides: Partial<Item>): Item {
  return { id: 'item-1', title: 'Track', sequence: '1', runtimeSeconds: 0, status: ItemStatus.MISSING, ...overrides } as Item
}

describe('Tracklist', () => {
  it('never shows the play icon for a non-imported track — icon presence alone communicates state', () => {
    const mockTransport = createRouterTransport(() => {})
    renderTracklist({ tracks: [track({ id: 't1', title: 'Second Hand News', status: ItemStatus.MISSING })] }, mockTransport)

    expect(screen.getByText('Second Hand News')).toBeInTheDocument()
    expect(screen.getByText('Missing')).toBeInTheDocument()
    expect(screen.queryByLabelText('Play Second Hand News')).not.toBeInTheDocument()
  })

  it('shows a play button only for an imported track with a real MediaFile', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MediaFileService, {
        listMediaFiles: req => ({ mediaFiles: req.itemId === 'has-file' ? [{ id: 'mf-1' }] : [] }),
      })
    })
    renderTracklist(
      {
        tracks: [
          track({ id: 'has-file', title: 'Dreams', status: ItemStatus.IMPORTED }),
          track({ id: 'no-file', title: 'Songbird', status: ItemStatus.IMPORTED }),
        ],
      },
      mockTransport,
    )

    await screen.findByLabelText('Play Dreams')
    // The imported track with no MediaFile gets no play button.
    expect(screen.queryByLabelText('Play Songbird')).not.toBeInTheDocument()
  })

  it('clicking Play queues the following playable tracks and plays the clicked one', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MediaFileService, {
        listMediaFiles: req => ({ mediaFiles: req.itemId === 'no-file' ? [] : [{ id: 'mf-1' }] }),
      })
    })
    renderTracklist(
      {
        tracks: [
          track({ id: 't1', sequence: '1', title: 'One', status: ItemStatus.IMPORTED }),
          track({ id: 't2', sequence: '2', title: 'Two', status: ItemStatus.IMPORTED }),
          track({ id: 'no-file', sequence: '3', title: 'Three', status: ItemStatus.IMPORTED }),
          track({ id: 't4', sequence: '4', title: 'Four', status: ItemStatus.IMPORTED }),
        ],
      },
      mockTransport,
    )

    fireEvent.click(await screen.findByLabelText('Play One'))

    const probe = screen.getByTestId('player-probe')
    expect(probe).toHaveAttribute('data-current', 'One')
    // "Three" has no MediaFile and is skipped from the queued tail.
    expect(probe).toHaveAttribute('data-queue', 'Two,Four')
  })

  it('"Populate from MusicBrainz" is absent when the edition has no mbid', () => {
    const mockTransport = createRouterTransport(() => {})
    renderTracklist({ mbid: '', tracks: [] }, mockTransport)

    expect(screen.queryByText('Populate from MusicBrainz')).not.toBeInTheDocument()
  })

  it('"Populate from MusicBrainz" is absent once the tracklist is non-empty', () => {
    const mockTransport = createRouterTransport(() => {})
    renderTracklist({ mbid: 'release-mbid', tracks: [track({})] }, mockTransport)

    expect(screen.queryByText('Populate from MusicBrainz')).not.toBeInTheDocument()
  })

  it('populates every track across every medium and reports per-track progress, then refetches', async () => {
    const createdTitles: string[] = []
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        getRelease: req => {
          expect(req.mbid).toBe('release-mbid')
          return {
            media: [
              { position: 1, format: 'CD', tracks: [{ position: 1, number: '1', title: 'Track One', lengthMs: 200000, recordingMbid: 'rec-1' }] },
              { position: 2, format: 'CD', tracks: [{ position: 1, number: '1', title: 'Track Two', lengthMs: 180000, recordingMbid: '' }] },
            ],
          }
        },
      })
      router.service(MusicReleaseService, {
        createMusicReleaseTrack: req => {
          createdTitles.push(req.title)
          return { track: { id: `item-${req.title}`, title: req.title } }
        },
      })
    })

    const { refetch } = renderTracklist({ mbid: 'release-mbid', tracks: [] }, mockTransport)

    fireEvent.click(screen.getByText('Populate from MusicBrainz'))

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('2 of 2 tracks added'))
    expect(createdTitles).toEqual(['Track One', 'Track Two'])
    expect(refetch).toHaveBeenCalled()
  })

  it('a partial populate failure leaves successes in place and surfaces which tracks failed', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        getRelease: () => ({
          media: [
            { position: 1, format: 'CD', tracks: [{ position: 1, number: '1', title: 'Good Track', lengthMs: 200000, recordingMbid: '' }] },
            { position: 1, format: 'CD', tracks: [{ position: 2, number: '2', title: 'Bad Track', lengthMs: 180000, recordingMbid: '' }] },
          ],
        }),
      })
      router.service(MusicReleaseService, {
        createMusicReleaseTrack: req => {
          if (req.title === 'Bad Track') throw new Error('network drop')
          return { track: { id: 'item-good', title: req.title } }
        },
      })
    })

    renderTracklist({ mbid: 'release-mbid', tracks: [] }, mockTransport)

    fireEvent.click(screen.getByText('Populate from MusicBrainz'))

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent("Couldn't add: Bad Track"))
    expect(screen.getByRole('status')).toHaveTextContent('1 of 2 tracks added')
  })

  it('Add track opens AddTrackDialog and a successful add refetches the tracklist', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        createMusicReleaseTrack: req => ({ track: { id: 'item-new', title: req.title } }),
      })
    })
    const { refetch } = renderTracklist({ tracks: [] }, mockTransport)

    fireEvent.click(screen.getByText('Add track'))
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'New Track' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(refetch).toHaveBeenCalled())
    expect(screen.queryByRole('dialog', { name: 'Add Track Manually' })).not.toBeInTheDocument()
  })

  it('the Delete action opens TrackDeleteDialog and a confirmed delete refetches the tracklist', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        getItemDeletionImpact: () => ({ impacts: [] }),
        deleteItem: () => ({}),
      })
    })
    const { refetch } = renderTracklist({ tracks: [track({ id: 't1', title: 'Dreams' })] }, mockTransport)

    fireEvent.click(screen.getByLabelText('Delete Dreams'))
    await screen.findByText('Nothing else references this track.')
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(refetch).toHaveBeenCalled())
  })
})
