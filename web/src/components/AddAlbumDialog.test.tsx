import { createRouterTransport, ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { AddAlbumDialog } from './AddAlbumDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend. useAddAlbum's own composition logic (hit/win/lose, the
// concurrent-add race) is covered by useAddAlbum.test.tsx; this file only
// proves the dialog wires the discography browse → selection →
// useAddAlbum → onAdded correctly.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <AddAlbumDialog artistId="artist-1" artistMbid="artist-mbid-1" onClose={onClose} onAdded={onAdded} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

describe('AddAlbumDialog', () => {
  it('lists the artist discography and adds the picked album', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        listReleaseGroupsForArtist: req => {
          expect(req.artistMbid).toBe('artist-mbid-1')
          return { releaseGroups: [{ mbid: 'rg-1', title: 'Hi Infidelity', primaryType: 'Album' }] }
        },
      })
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(GroupService, {
        createGroup: req => {
          expect(req.group?.libraryEntryId).toBe('artist-1')
          return { group: { id: 'new-1', title: 'Hi Infidelity' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    await waitFor(() => expect(screen.getByRole('button', { name: /Hi Infidelity/ })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Hi Infidelity/ }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'new-1' })))
  })

  it('shows "No albums found" for a genuinely empty discography', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        listReleaseGroupsForArtist: () => ({ releaseGroups: [] }),
      })
    })
    renderDialog(mockTransport)

    await waitFor(() => expect(screen.getByText('No albums found')).toBeInTheDocument())
  })

  it('shows an inline error and stays open when the add composition fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        listReleaseGroupsForArtist: () => ({ releaseGroups: [{ mbid: 'rg-1', title: 'Hi Infidelity', primaryType: 'Album' }] }),
      })
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('lookup failed', Code.Internal)
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    await waitFor(() => expect(screen.getByRole('button', { name: /Hi Infidelity/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Hi Infidelity/ }))

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(onAdded).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
