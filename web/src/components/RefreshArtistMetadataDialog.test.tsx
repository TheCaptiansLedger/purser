import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { RefreshArtistMetadataDialog } from './RefreshArtistMetadataDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend. useRefreshArtistMetadata's own diff/apply logic is
// covered by useRefreshArtistMetadata.test.tsx; this file only proves the
// dialog renders that hook's state and wires Confirm/Cancel correctly.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>, entry: LibraryEntry) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onUpdated = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <RefreshArtistMetadataDialog entry={entry} mbid="mbid-1" onClose={onClose} onUpdated={onUpdated} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onUpdated }
}

const entry: LibraryEntry = {
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
  metadata: { artist_type: 'Group' },
}

describe('RefreshArtistMetadataDialog', () => {
  it('renders "Already up to date" and no Confirm button when nothing differs', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        getArtist: () => ({
          artist: { mbid: 'mbid-1', name: 'REO Speedwagon', sortName: 'REO Speedwagon', type: 'Group', aliases: [] },
          isnis: [],
          officialUrl: '',
          wikipediaUrl: '',
          members: [],
        }),
      })
    })
    renderDialog(mockTransport, entry)

    await waitFor(() => expect(screen.getByText('Already up to date.')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Confirm' })).not.toBeInTheDocument()
  })

  it('lists changed fields and applies exactly that diff on Confirm', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        getArtist: () => ({
          artist: { mbid: 'mbid-1', name: 'R.E.O. Speedwagon', sortName: 'REO Speedwagon', type: 'Group', aliases: [] },
          isnis: [],
          officialUrl: '',
          wikipediaUrl: '',
          members: [],
        }),
      })
      router.service(LibraryEntryService, {
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['name'])
          return { libraryEntry: { ...entry, name: 'R.E.O. Speedwagon' } }
        },
      })
    })
    const { onClose, onUpdated } = renderDialog(mockTransport, entry)

    await waitFor(() => expect(screen.getByText('Name')).toBeInTheDocument())
    expect(screen.getByText('R.E.O. Speedwagon')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() => expect(onUpdated).toHaveBeenCalledWith(expect.objectContaining({ name: 'R.E.O. Speedwagon' })))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows an inline error and stays open when the update fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        getArtist: () => ({
          artist: { mbid: 'mbid-1', name: 'R.E.O. Speedwagon', sortName: 'REO Speedwagon', type: 'Group', aliases: [] },
          isnis: [],
          officialUrl: '',
          wikipediaUrl: '',
          members: [],
        }),
      })
      router.service(LibraryEntryService, {
        updateLibraryEntry: () => {
          throw new Error('boom')
        },
      })
    })
    const { onClose, onUpdated } = renderDialog(mockTransport, entry)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(onUpdated).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
