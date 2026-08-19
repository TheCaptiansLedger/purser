import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import { useGroupsByIds } from '../hooks/useGroupsByIds'
import { useItemsByStatus } from '../hooks/useItemsByStatus'
import { useLibraryEntriesByIds } from '../hooks/useLibraryEntriesByIds'
import { WantedBoard } from './WantedBoard'

// Page-level test: composition only, per ADR 0004 — useItemsByStatus' own
// wire behavior is tested in useItemsByStatus.test.tsx, useGroupsByIds'/
// useLibraryEntriesByIds' own fan-out in their own test files. All are
// mocked directly so every state is reachable deterministically and this
// page doesn't need a mocked transport of its own.
vi.mock('../hooks/useItemsByStatus')
const mockUseItemsByStatus = vi.mocked(useItemsByStatus)

vi.mock('../hooks/useGroupsByIds')
const mockUseGroupsByIds = vi.mocked(useGroupsByIds)
mockUseGroupsByIds.mockReturnValue({ groupsById: {}, isPending: false })

vi.mock('../hooks/useLibraryEntriesByIds')
const mockUseLibraryEntriesByIds = vi.mocked(useLibraryEntriesByIds)
mockUseLibraryEntriesByIds.mockReturnValue({ entriesById: {}, isPending: false })

function renderWantedBoard() {
  render(
    <MemoryRouter initialEntries={['/music/wanted']}>
      <Routes>
        <Route path="/music/wanted" element={<WantedBoard />} />
        <Route path="/music/artists/:id" element={<div>Artist detail page</div>} />
        <Route path="/music/albums/:id" element={<div>Album detail page</div>} />
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
  } as unknown as ReturnType<typeof useItemsByStatus>
}

function loaded(
  items: { id: string; title: string; libraryEntryId: string; groupId: string; status: ItemStatus }[],
  hasNextPage = false,
) {
  return {
    data: { pages: [{ items, nextPageToken: hasNextPage ? 'next' : '' }] },
    isPending: false,
    isError: false,
    error: null,
    hasNextPage,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  } as unknown as ReturnType<typeof useItemsByStatus>
}

describe('WantedBoard', () => {
  it('queries content_type="music" with the Wanted status for the default tab', () => {
    mockUseItemsByStatus.mockReturnValue(loaded([]))

    renderWantedBoard()

    expect(mockUseItemsByStatus).toHaveBeenCalledWith(ItemStatus.WANTED)
  })

  it('re-queries with the new status when switching tabs, not a client-side filter', () => {
    mockUseItemsByStatus.mockReturnValue(loaded([]))

    renderWantedBoard()
    fireEvent.click(screen.getByRole('tab', { name: 'Missing' }))

    expect(mockUseItemsByStatus).toHaveBeenLastCalledWith(ItemStatus.MISSING)
  })

  it('shows EmptyState, not a blank table, when the active tab has nothing', () => {
    mockUseItemsByStatus.mockReturnValue(loaded([]))

    renderWantedBoard()

    expect(screen.getByText('Nothing Wanted')).toBeInTheDocument()
  })

  it('renders each item resolved to its artist and album via Group/LibraryEntry lookups', () => {
    mockUseItemsByStatus.mockReturnValue(
      loaded([{ id: 'i1', title: 'Dreams', libraryEntryId: 'artist-1', groupId: 'group-1', status: ItemStatus.WANTED }]),
    )
    mockUseGroupsByIds.mockReturnValue({
      groupsById: { 'group-1': { id: 'group-1', title: 'Rumours' } },
      isPending: false,
    } as unknown as ReturnType<typeof useGroupsByIds>)
    mockUseLibraryEntriesByIds.mockReturnValue({
      entriesById: { 'artist-1': { id: 'artist-1', name: 'Fleetwood Mac' } },
      isPending: false,
    } as unknown as ReturnType<typeof useLibraryEntriesByIds>)

    renderWantedBoard()

    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.getByText('Rumours')).toBeInTheDocument()
    expect(screen.getByText('Dreams')).toBeInTheDocument()
  })

  it('links each row to its Artist Detail and Album Detail routes', () => {
    mockUseItemsByStatus.mockReturnValue(
      loaded([{ id: 'i1', title: 'Dreams', libraryEntryId: 'artist-1', groupId: 'group-1', status: ItemStatus.WANTED }]),
    )
    mockUseGroupsByIds.mockReturnValue({
      groupsById: { 'group-1': { id: 'group-1', title: 'Rumours' } },
      isPending: false,
    } as unknown as ReturnType<typeof useGroupsByIds>)
    mockUseLibraryEntriesByIds.mockReturnValue({
      entriesById: { 'artist-1': { id: 'artist-1', name: 'Fleetwood Mac' } },
      isPending: false,
    } as unknown as ReturnType<typeof useLibraryEntriesByIds>)

    renderWantedBoard()
    fireEvent.click(screen.getByText('Rumours'))

    expect(screen.getByText('Album detail page')).toBeInTheDocument()
  })

  it('shows nothing while the tab is pending, not a premature EmptyState', () => {
    mockUseItemsByStatus.mockReturnValue(pending())

    renderWantedBoard()

    expect(screen.queryByText('Nothing Wanted')).not.toBeInTheDocument()
  })

  it('shows an actionable error when ListItems fails', () => {
    mockUseItemsByStatus.mockReturnValue({
      ...pending(),
      isPending: false,
      isError: true,
      error: { message: 'unavailable' },
    } as unknown as ReturnType<typeof useItemsByStatus>)

    renderWantedBoard()

    expect(screen.getByText(/unavailable/)).toBeInTheDocument()
  })

  it('calls fetchNextPage from Load more', () => {
    const listResult = loaded([{ id: 'i1', title: 'Dreams', libraryEntryId: 'artist-1', groupId: 'group-1', status: ItemStatus.WANTED }], true)
    mockUseItemsByStatus.mockReturnValue(listResult)

    renderWantedBoard()
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))

    expect(listResult.fetchNextPage).toHaveBeenCalledOnce()
  })
})
