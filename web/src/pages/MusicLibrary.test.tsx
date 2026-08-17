import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { useArtistLibraryEntries } from '../hooks/useArtistLibraryEntries'
import { useLibraryEntryImages } from '../hooks/useLibraryEntryImages'
import { MusicLibrary } from './MusicLibrary'
import { AddArtistDialog, type AddArtistDialogProps } from '../components/AddArtistDialog'

// Page-level test: composition only, per ADR 0004 — useArtistLibraryEntries'
// own wire behavior is tested in useArtistLibraryEntries.test.tsx,
// useLibraryEntryImages' own fan-out in useLibraryEntryImages.test.tsx,
// ArtistCard's own rendering in ArtistCard.test.tsx, AddArtistDialog's own
// search/select/error behavior in AddArtistDialog.test.tsx. All three are
// mocked directly so every state is reachable deterministically and this
// page doesn't need a mocked transport of its own.
vi.mock('../hooks/useArtistLibraryEntries')
const mockUseArtistLibraryEntries = vi.mocked(useArtistLibraryEntries)

vi.mock('../hooks/useLibraryEntryImages')
const mockUseLibraryEntryImages = vi.mocked(useLibraryEntryImages)
mockUseLibraryEntryImages.mockReturnValue({})

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

function renderMusicLibrary() {
  render(
    <MemoryRouter initialEntries={['/music']}>
      <Routes>
        <Route path="/music" element={<MusicLibrary />} />
        <Route path="/music/artists/:id" element={<div>Artist detail page</div>} />
      </Routes>
    </MemoryRouter>,
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

  it('opens AddArtistDialog from the Add Artist button, and closing it dismisses the dialog', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([]))

    renderMusicLibrary()
    expect(screen.queryByText('Add Artist dialog open')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Artist' }))
    expect(screen.getByText('Add Artist dialog open')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'stub-close' }))
    expect(screen.queryByText('Add Artist dialog open')).not.toBeInTheDocument()
  })

  it('navigates to the new artist and closes the dialog when AddArtistDialog reports success', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([]))

    renderMusicLibrary()
    fireEvent.click(screen.getByRole('button', { name: 'Add Artist' }))
    fireEvent.click(screen.getByRole('button', { name: 'stub-add' }))

    expect(screen.getByText('Artist detail page')).toBeInTheDocument()
    expect(screen.queryByText('Add Artist dialog open')).not.toBeInTheDocument()
  })

  it('links each card to its Artist Detail route', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: true }]))

    renderMusicLibrary()
    fireEvent.click(screen.getByText('Fleetwood Mac'))

    expect(screen.getByText('Artist detail page')).toBeInTheDocument()
  })
})
