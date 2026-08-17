import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useArtistLibraryEntries } from '../hooks/useArtistLibraryEntries'
import { useLibraryEntryImages } from '../hooks/useLibraryEntryImages'
import { MusicLibrary } from './MusicLibrary'

// Page-level test: composition only, per ADR 0004 — useArtistLibraryEntries'
// own wire behavior is tested in useArtistLibraryEntries.test.tsx,
// useLibraryEntryImages' own fan-out in useLibraryEntryImages.test.tsx,
// ArtistCard's own rendering in ArtistCard.test.tsx. Both hooks are
// mocked directly so every state is reachable deterministically.
vi.mock('../hooks/useArtistLibraryEntries')
const mockUseArtistLibraryEntries = vi.mocked(useArtistLibraryEntries)

vi.mock('../hooks/useLibraryEntryImages')
const mockUseLibraryEntryImages = vi.mocked(useLibraryEntryImages)
mockUseLibraryEntryImages.mockReturnValue({})

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

    render(<MusicLibrary />)

    expect(screen.getByText('No artists yet')).toBeInTheDocument()
  })

  it('renders an ArtistCard grid for a loaded page, including its genre subtitle', () => {
    mockUseArtistLibraryEntries.mockReturnValue(
      loaded([
        { id: 'a1', name: 'Fleetwood Mac', monitored: true, metadata: { genre: 'Rock' } },
        { id: 'a2', name: 'Steely Dan', monitored: false },
      ]),
    )

    render(<MusicLibrary />)

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

    render(<MusicLibrary />)
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

    render(<MusicLibrary />)
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.queryByText('Steely Dan')).not.toBeInTheDocument()
  })

  it('shows the zero-results EmptyState when a filter matches nothing', () => {
    mockUseArtistLibraryEntries.mockReturnValue(loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: false }]))

    render(<MusicLibrary />)
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('No matches')).toBeInTheDocument()
  })

  it('calls fetchNextPage from Load more without unmounting the existing grid', () => {
    const listResult = loaded([{ id: 'a1', name: 'Fleetwood Mac', monitored: true }], true)
    mockUseArtistLibraryEntries.mockReturnValue(listResult)

    render(<MusicLibrary />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))

    expect(listResult.fetchNextPage).toHaveBeenCalledOnce()
    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
  })

  it('shows nothing while the first page is pending', () => {
    mockUseArtistLibraryEntries.mockReturnValue(pending())

    render(<MusicLibrary />)

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

    render(<MusicLibrary />)

    expect(screen.getByText(/unavailable/)).toBeInTheDocument()
  })
})
