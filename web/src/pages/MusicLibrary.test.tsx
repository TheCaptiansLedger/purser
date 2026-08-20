import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { useArtistLibraryEntries } from '../hooks/useArtistLibraryEntries'
import { useLibraryEntryDeletionImpacts } from '../hooks/useLibraryEntryDeletionImpacts'
import { useLibraryEntryImages } from '../hooks/useLibraryEntryImages'
import { useLibraryOwnership } from '../hooks/useLibraryOwnership'
import { MusicLibrary } from './MusicLibrary'
import { AddArtistDialog, type AddArtistDialogProps } from '../components/AddArtistDialog'
import { ManualArtistDialog, type ManualArtistDialogProps } from '../components/ManualArtistDialog'

// Page-level test: composition only, per ADR 0004 — useArtistLibraryEntries'
// own wire behavior is tested in useArtistLibraryEntries.test.tsx,
// useLibraryEntryImages' own fan-out in useLibraryEntryImages.test.tsx,
// useLibraryOwnership's own fan-out in useLibraryOwnership.test.tsx,
// useLibraryEntryDeletionImpacts' own fan-out in its own test,
// ArtistCard's own rendering in ArtistCard.test.tsx, AddArtistDialog's own
// search/select/error behavior in AddArtistDialog.test.tsx,
// BulkDeleteDialog's own blocking/cascade logic in its own test. All are
// mocked directly so every state is reachable deterministically. The one
// exception is BulkDeleteLibraryEntries itself (#679): that RPC call is
// exercised through a real mocked transport, since the point of this
// page's bulk-delete tests is proving MusicLibrary wires the right
// selected ids/cascade flag through, not just that some mock fired.
vi.mock('../hooks/useArtistLibraryEntries')
const mockUseArtistLibraryEntries = vi.mocked(useArtistLibraryEntries)

vi.mock('../hooks/useLibraryEntryImages')
const mockUseLibraryEntryImages = vi.mocked(useLibraryEntryImages)
mockUseLibraryEntryImages.mockReturnValue({})

vi.mock('../hooks/useLibraryOwnership')
const mockUseLibraryOwnership = vi.mocked(useLibraryOwnership)
mockUseLibraryOwnership.mockReturnValue({})

vi.mock('../hooks/useLibraryEntryDeletionImpacts')
const mockUseLibraryEntryDeletionImpacts = vi.mocked(useLibraryEntryDeletionImpacts)
mockUseLibraryEntryDeletionImpacts.mockReturnValue({ isPending: false, rows: [] })

vi.mock('../components/AddArtistDialog')
const mockAddArtistDialog = vi.mocked(AddArtistDialog)
// A minimal stand-in that exposes onClose/onAdded as clickable buttons so
// tests can drive MusicLibrary's own reaction to the dialog's outcome
// without exercising AddArtistDialog's real search behavior.
mockAddArtistDialog.mockImplementation(({ onClose, onAdded }: AddArtistDialogProps) => (
  <div>
    <span>Add Artist dialog open</span>
    <button type="button" onClick={onClose}>
      stub-close
    </button>
    <button type="button" onClick={() => onAdded({ id: 'new-artist-1' } as LibraryEntry)}>
      stub-add
    </button>
  </div>
))

vi.mock('../components/ManualArtistDialog')
const mockManualArtistDialog = vi.mocked(ManualArtistDialog)
// Same minimal stand-in shape as AddArtistDialog's mock above, so
// MusicLibrary's own reaction to either entry point is driven the same
// way — ManualArtistDialog's real form behavior lives in its own test.
mockManualArtistDialog.mockImplementation(({ onClose, onAdded }: ManualArtistDialogProps) => (
  <div>
    <span>Manual Artist dialog open</span>
    <button type="button" onClick={onClose}>
      stub-manual-close
    </button>
    <button type="button" onClick={() => onAdded({ id: 'new-artist-2' } as LibraryEntry)}>
      stub-manual-add
    </button>
  </div>
))

function renderMusicLibrary(mockTransport: ReturnType<typeof createRouterTransport> = createRouterTransport(() => {})) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/music']}>
          <Routes>
            <Route path="/music" element={<MusicLibrary />} />
            <Route path="/music/artists/:id" element={<div>Artist detail page</div>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </TransportProvider>,
  )
}

function pending() {
  return {
    data: undefined,
    isPending: true,
    isError: false,
    error: null,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useArtistLibraryEntries>
}

function loaded(
  libraryEntries: { id: string; name: string; monitored: boolean; metadata?: { genre?: string } }[],
  hasNextPage = false,
) {
  return {
    data: { pages: [{ libraryEntries, nextPageToken: hasNextPage ? 'next' : '' }] },
    isPending: false,
    isError: false,
    error: null,
    hasNextPage,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useArtistLibraryEntries>
}

describe('MusicLibrary', () => {
  it('shows the empty-library state when nothing exists', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([]))

    renderMusicLibrary()

    expect(screen.getByText('No artists yet')).toBeInTheDocument()
  })

  it('renders an ArtistCard grid for a loaded page, including its genre subtitle', () => {
    mockUseArtistLibraryEntries.mockReturnValue(
      loaded([
        { id: 'a1', name: 'Fleetwood Mac', monitored: true, metadata: { genre: 'Rock' } },
        { id: 'a2', name: 'Steely Dan', monitored: false },
      ]),
    )

    renderMusicLibrary()

    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.getByText('Rock')).toBeInTheDocument()
    expect(screen.getByText('Steely Dan')).toBeInTheDocument()
    expect(screen.queryByText('No artists yet')).not.toBeInTheDocument()
  })

  it('filters the loaded page client-side by name as the search box changes', () => {
    mockUseArtistLibraryEntries.mockReturnValue(
      loaded([
        { id: 'a1', name: 'Fleetwood Mac', monitored: true },
        { id: 'a2', name: 'Steely Dan', monitored: true },
      ]),
    )

    renderMusicLibrary()
    fireEvent.change(screen.getByLabelText('Search artists'), { target: { value: 'fleet' } })

    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.queryByText('Steely Dan')).not.toBeInTheDocument()
  })

  it('filters the loaded page client-side when Monitored only is checked', () => {
    mockUseArtistLibraryEntries.mockReturnValue(
      loaded([
        { id: 'a1', name: 'Fleetwood Mac', monitored: true },
        { id: 'a2', name: 'Steely Dan', monitored: false },
      ]),
    )

    renderMusicLibrary()
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.queryByText('Steely Dan')).not.toBeInTheDocument()
  })

  it('shows the zero-results EmptyState when a filter matches nothing', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: false }]))

    renderMusicLibrary()
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('No matches')).toBeInTheDocument()
  })

  it('calls fetchNextPage from Load more without unmounting the existing grid', () => {
    const listResult = loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: true }], true)
    mockUseArtistLibraryEntries.mockReturnValue(listResult)

    renderMusicLibrary()
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))

    expect(listResult.fetchNextPage).toHaveBeenCalledOnce()
    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
  })

  it('shows nothing while the first page is pending', () => {
    mockUseArtistLibraryEntries.mockReturnValue(pending())

    renderMusicLibrary()

    expect(screen.queryByText('No artists yet')).not.toBeInTheDocument()
    expect(screen.queryByText('No matches')).not.toBeInTheDocument()
  })

  it('shows an actionable error when ListLibraryEntries fails', () => {
    mockUseArtistLibraryEntries.mockReturnValue({
      ...pending(),
      isPending: false,
      isError: true,
      error: { message: 'unavailable' },
    } as unknown as ReturnType<typeof useArtistLibraryEntries>)

    renderMusicLibrary()

    expect(screen.getByText(/unavailable/)).toBeInTheDocument()
  })

  it('opens AddArtistDialog from Add Artist → Search MusicBrainz, and closing it dismisses the dialog', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([]))

    renderMusicLibrary()
    expect(screen.queryByText('Add Artist dialog open')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Artist' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Search MusicBrainz' }))
    expect(screen.getByText('Add Artist dialog open')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'stub-close' }))
    expect(screen.queryByText('Add Artist dialog open')).not.toBeInTheDocument()
  })

  it('navigates to the new artist and closes the dialog when AddArtistDialog reports success', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([]))

    renderMusicLibrary()
    fireEvent.click(screen.getByRole('button', { name: 'Add Artist' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Search MusicBrainz' }))
    fireEvent.click(screen.getByRole('button', { name: 'stub-add' }))

    expect(screen.getByText('Artist detail page')).toBeInTheDocument()
    expect(screen.queryByText('Add Artist dialog open')).not.toBeInTheDocument()
  })

  it('opens ManualArtistDialog from Add Artist → Add Manually, and closing it dismisses the dialog', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([]))

    renderMusicLibrary()
    expect(screen.queryByText('Manual Artist dialog open')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Artist' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Add Manually' }))
    expect(screen.getByText('Manual Artist dialog open')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'stub-manual-close' }))
    expect(screen.queryByText('Manual Artist dialog open')).not.toBeInTheDocument()
  })

  it('navigates to the new artist and closes the dialog when ManualArtistDialog reports success', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([]))

    renderMusicLibrary()
    fireEvent.click(screen.getByRole('button', { name: 'Add Artist' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Add Manually' }))
    fireEvent.click(screen.getByRole('button', { name: 'stub-manual-add' }))

    expect(screen.getByText('Artist detail page')).toBeInTheDocument()
    expect(screen.queryByText('Manual Artist dialog open')).not.toBeInTheDocument()
  })

  it("passes each artist's own useLibraryOwnership result into its ArtistCard", () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: true }]))
    mockUseLibraryOwnership.mockReturnValue({ a1: { owned: 4, total: 7 } })

    renderMusicLibrary()

    expect(screen.getByRole('img', { name: '4 of 7 albums owned' })).toBeInTheDocument()
  })

  it('links each card to its Artist Detail route', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: true }]))

    renderMusicLibrary()
    fireEvent.click(screen.getByText('Fleetwood Mac'))

    expect(screen.getByText('Artist detail page')).toBeInTheDocument()
  })

  describe('bulk delete (#679)', () => {
    it('Select swaps card navigation for toggle-selection, tracked by the SelectionToolbar', () => {
      mockUseArtistLibraryEntries.mockReturnValue(
        loaded([
          { id: 'a1', name: 'Fleetwood Mac', monitored: true },
          { id: 'a2', name: 'Steely Dan', monitored: true },
        ]),
      )

      renderMusicLibrary()
      fireEvent.click(screen.getByRole('button', { name: 'Select' }))
      expect(screen.getByText('0 artists selected')).toBeInTheDocument()

      fireEvent.click(screen.getByText('Fleetwood Mac'))
      expect(screen.getByText('1 artists selected')).toBeInTheDocument()
      expect(screen.queryByText('Artist detail page')).not.toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      expect(screen.queryByText('1 artists selected')).not.toBeInTheDocument()
    })

    it('opens BulkDeleteDialog from Delete, showing the aggregated impact', () => {
      mockUseArtistLibraryEntries.mockReturnValue(loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: true }]))
      mockUseLibraryEntryDeletionImpacts.mockReturnValue({
        isPending: false,
        rows: [{ kind: 'group', label: 'Groups', count: 3, blocking: true }],
      })

      renderMusicLibrary()
      fireEvent.click(screen.getByRole('button', { name: 'Select' }))
      fireEvent.click(screen.getByText('Fleetwood Mac'))
      fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

      expect(screen.getByRole('dialog', { name: 'Delete 1 artists?' })).toBeInTheDocument()
      expect(screen.getByText('Groups')).toBeInTheDocument()
    })

    it('confirming the dialog calls BulkDeleteLibraryEntries with the selected ids and cascade, then exits select mode', async () => {
      mockUseArtistLibraryEntries.mockReturnValue(loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: true }]))
      mockUseLibraryEntryDeletionImpacts.mockReturnValue({ isPending: false, rows: [] })

      let bulkDeleteRequest: { ids: string[]; cascade: boolean } | undefined
      const mockTransport = createRouterTransport(router => {
        router.service(LibraryEntryService, {
          bulkDeleteLibraryEntries: request => {
            bulkDeleteRequest = { ids: request.ids, cascade: request.cascade }
            return {}
          },
        })
      })

      renderMusicLibrary(mockTransport)
      fireEvent.click(screen.getByRole('button', { name: 'Select' }))
      fireEvent.click(screen.getByText('Fleetwood Mac'))
      fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
      fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }))

      await waitFor(() => expect(bulkDeleteRequest).toEqual({ ids: ['a1'], cascade: false }))
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(screen.queryByText('1 artists selected')).not.toBeInTheDocument()
    })
  })

  describe('bulk monitor toggle (#680)', () => {
    it('calls UpdateLibraryEntry for every selected artist and refetches on success', async () => {
      const loadedResult = loaded([
        { id: 'a1', name: 'Fleetwood Mac', monitored: false },
        { id: 'a2', name: 'Steely Dan', monitored: false },
      ])
      mockUseArtistLibraryEntries.mockReturnValue(loadedResult)

      const requestedIds: string[] = []
      const mockTransport = createRouterTransport(router => {
        router.service(LibraryEntryService, {
          updateLibraryEntry: request => {
            requestedIds.push(request.libraryEntry!.id)
            return { libraryEntry: request.libraryEntry }
          },
        })
      })

      renderMusicLibrary(mockTransport)
      fireEvent.click(screen.getByRole('button', { name: 'Select' }))
      fireEvent.click(screen.getByText('Fleetwood Mac'))
      fireEvent.click(screen.getByText('Steely Dan'))
      fireEvent.click(screen.getByRole('button', { name: 'Monitor' }))

      await waitFor(() => expect(requestedIds.sort()).toEqual(['a1', 'a2']))
      await waitFor(() => expect(loadedResult.refetch).toHaveBeenCalled())
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })

    it('surfaces a per-row failure via BulkActionErrors without touching the successful row', async () => {
      mockUseArtistLibraryEntries.mockReturnValue(
        loaded([
          { id: 'a1', name: 'Fleetwood Mac', monitored: false },
          { id: 'a2', name: 'Steely Dan', monitored: false },
        ]),
      )

      const mockTransport = createRouterTransport(router => {
        router.service(LibraryEntryService, {
          updateLibraryEntry: request => {
            if (request.libraryEntry!.id === 'a2') {
              throw new Error('boom')
            }
            return { libraryEntry: request.libraryEntry }
          },
        })
      })

      renderMusicLibrary(mockTransport)
      fireEvent.click(screen.getByRole('button', { name: 'Select' }))
      fireEvent.click(screen.getByText('Fleetwood Mac'))
      fireEvent.click(screen.getByText('Steely Dan'))
      fireEvent.click(screen.getByRole('button', { name: 'Unmonitor' }))

      await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't update 1 item"))
      expect(screen.getByText(/Steely Dan:/)).toBeInTheDocument()
      expect(screen.queryByText(/Fleetwood Mac:/)).not.toBeInTheDocument()
    })
  })
})
