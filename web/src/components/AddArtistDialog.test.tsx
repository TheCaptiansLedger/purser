import { createRouterTransport, ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { TheAudioDBService } from '../gen/purser/music/v1/theaudiodb_pb'
import { AddArtistDialog } from './AddArtistDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// ChooseArtworkDialog.test.tsx. useAddArtist's own composition logic
// (hit/win/lose/TheAudioDB best-effort, the concurrent-add race) is
// covered by useAddArtist.test.tsx; this file only proves the dialog
// wires search → selection → useAddArtist → onAdded correctly.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <AddArtistDialog onClose={onClose} onAdded={onAdded} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

function tadbMiss(router: Parameters<Parameters<typeof createRouterTransport>[0]>[0]) {
  router.service(TheAudioDBService, {
    lookupArtist: () => {
      throw new ConnectError('not found', Code.NotFound)
    },
  })
}

describe('AddArtistDialog', () => {
  it('searches MusicBrainz, lists results, and adds the picked artist', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        searchArtists: req => {
          expect(req.query).toBe('reo speedwagon')
          return { artists: [{ mbid: 'mbid-1', name: 'REO Speedwagon', sortName: 'REO Speedwagon', type: 'Group' }] }
        },
      })
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(LibraryEntryService, {
        createLibraryEntry: () => ({ libraryEntry: { id: 'new-1', name: 'REO Speedwagon' } }),
      })
      tadbMiss(router)
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Search MusicBrainz for an artist'), {
      target: { value: 'reo speedwagon' },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: /REO Speedwagon/ })).toBeInTheDocument(), {
      timeout: 2000,
    })

    fireEvent.click(screen.getByRole('button', { name: /REO Speedwagon/ }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'new-1' })))
  })

  it('shows "No matches" for a genuinely empty MusicBrainz result', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        searchArtists: () => ({ artists: [] }),
      })
    })
    renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Search MusicBrainz for an artist'), { target: { value: 'nobody' } })

    await waitFor(() => expect(screen.getByText('No matches')).toBeInTheDocument(), { timeout: 2000 })
  })

  it('shows an inline error and stays open when the add composition fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        searchArtists: () => ({ artists: [{ mbid: 'mbid-1', name: 'REO Speedwagon', sortName: 'REO Speedwagon', type: 'Group' }] }),
      })
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('lookup failed', Code.Internal)
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Search MusicBrainz for an artist'), { target: { value: 'reo' } })
    await waitFor(() => expect(screen.getByRole('button', { name: /REO Speedwagon/ })).toBeInTheDocument(), {
      timeout: 2000,
    })
    fireEvent.click(screen.getByRole('button', { name: /REO Speedwagon/ }))

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(onAdded).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
