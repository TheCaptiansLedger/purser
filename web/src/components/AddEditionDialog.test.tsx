import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { MusicReleaseService, ReleaseStatus } from '../gen/purser/music/v1/release_pb'
import { AddEditionDialog } from './AddEditionDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend. Unlike AddAlbumDialog, there is no separate composition
// hook to unit test — CreateMusicRelease's server-side get-or-create-on-
// MBID (music.Release, ADR 0021) is exercised at the Go layer; this file
// only proves the dialog wires the edition browse → selection →
// CreateMusicRelease → onAdded correctly.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <AddEditionDialog
          groupId="group-1"
          libraryEntryId="entry-1"
          releaseGroupMbid="rg-mbid-1"
          onClose={onClose}
          onAdded={onAdded}
        />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

describe('AddEditionDialog', () => {
  it('lists MusicBrainz editions and creates the picked one', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        listReleasesForReleaseGroup: req => {
          expect(req.releaseGroupMbid).toBe('rg-mbid-1')
          return {
            releases: [
              {
                mbid: 'release-mbid-1',
                title: 'Hi Infidelity',
                country: 'US',
                date: '1980-11-21',
                label: 'Epic',
                catalogNumber: 'E2 85369',
                barcode: '075992599720',
                format: 'CD',
                mediumCount: 1,
                trackCount: 10,
              },
            ],
          }
        },
      })
      router.service(MusicReleaseService, {
        createMusicRelease: req => {
          expect(req.musicRelease?.groupId).toBe('group-1')
          expect(req.musicRelease?.libraryEntryId).toBe('entry-1')
          expect(req.musicRelease?.mbid).toBe('release-mbid-1')
          expect(req.musicRelease?.catalogNumber).toBe('E2 85369')
          expect(req.musicRelease?.status).toBe(ReleaseStatus.STUB)
          expect(req.musicRelease?.isDefault).toBe(false)
          return { musicRelease: { ...req.musicRelease!, id: 'release-1' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    await waitFor(() => expect(screen.getByRole('button', { name: /Hi Infidelity/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Hi Infidelity/ }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'release-1' })))
  })

  it('shows "No editions found" for a genuinely empty edition list', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, { listReleasesForReleaseGroup: () => ({ releases: [] }) })
    })
    renderDialog(mockTransport)

    await waitFor(() => expect(screen.getByText('No editions found')).toBeInTheDocument())
  })

  it('shows an inline error and stays open when CreateMusicRelease fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        listReleasesForReleaseGroup: () => ({ releases: [{ mbid: 'release-mbid-1', title: 'Hi Infidelity' }] }),
      })
      router.service(MusicReleaseService, {
        createMusicRelease: () => {
          throw new Error('unavailable')
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
