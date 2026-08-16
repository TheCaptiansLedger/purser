import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { usePeopleList } from '../hooks/usePeopleList'
import { usePersonImages } from '../hooks/usePersonImages'
import { People } from './People'

// Page-level test: composition only, per ADR 0004 — usePeopleList's own
// wire behavior is tested in usePeopleList.test.tsx, usePersonImages' own
// fan-out in usePersonImages.test.tsx, PersonCard's own rendering in
// PersonCard.test.tsx, EmptyState's own rendering in EmptyState.test.tsx.
// Both hooks are mocked directly so every state is reachable
// deterministically.
vi.mock('../hooks/usePeopleList')
const mockUsePeopleList = vi.mocked(usePeopleList)

vi.mock('../hooks/usePersonImages')
const mockUsePersonImages = vi.mocked(usePersonImages)
mockUsePersonImages.mockReturnValue({})

function pending() {
  return {
    data: undefined,
    isPending: true,
    isError: false,
    error: null,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  } as unknown as ReturnType<typeof usePeopleList>
}

function loaded(people: { id: string; name: string; monitored: boolean }[], hasNextPage = false) {
  return {
    data: { pages: [{ people, nextPageToken: hasNextPage ? 'next' : '' }] },
    isPending: false,
    isError: false,
    error: null,
    hasNextPage,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  } as unknown as ReturnType<typeof usePeopleList>
}

describe('People', () => {
  it('shows the empty-library state with a disabled Add Person action when nothing exists', () => {
    mockUsePeopleList.mockReturnValue(loaded([]))

    render(<People />)

    expect(screen.getByText('No people yet')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add Person' })).toBeDisabled()
  })

  it('renders a PersonCard grid for a loaded page', () => {
    mockUsePeopleList.mockReturnValue(
      loaded([
        { id: 'p1', name: 'Jane Doe', monitored: true },
        { id: 'p2', name: 'Stevie Nicks', monitored: false },
      ]),
    )

    render(<People />)

    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
    expect(screen.getByText('Stevie Nicks')).toBeInTheDocument()
    expect(screen.queryByText('No people yet')).not.toBeInTheDocument()
  })

  it('filters the loaded page client-side when Monitored only is checked', () => {
    mockUsePeopleList.mockReturnValue(
      loaded([
        { id: 'p1', name: 'Jane Doe', monitored: true },
        { id: 'p2', name: 'Stevie Nicks', monitored: false },
      ]),
    )

    render(<People />)
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
    expect(screen.queryByText('Stevie Nicks')).not.toBeInTheDocument()
  })

  it('shows the zero-results EmptyState when Monitored only filters out every loaded person', () => {
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: false }]))

    render(<People />)
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('No matches')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Add Person' })).not.toBeInTheDocument()
  })

  it('calls fetchNextPage from Load more without unmounting the existing grid', () => {
    const listResult = loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }], true)
    mockUsePeopleList.mockReturnValue(listResult)

    render(<People />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))

    expect(listResult.fetchNextPage).toHaveBeenCalledOnce()
    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
  })

  it('shows nothing while the first page is pending', () => {
    mockUsePeopleList.mockReturnValue(pending())

    render(<People />)

    expect(screen.queryByText('No people yet')).not.toBeInTheDocument()
    expect(screen.queryByText('No matches')).not.toBeInTheDocument()
  })

  it('shows an actionable error when ListPeople fails', () => {
    mockUsePeopleList.mockReturnValue({
      ...pending(),
      isPending: false,
      isError: true,
      error: { message: 'unavailable' },
    } as unknown as ReturnType<typeof usePeopleList>)

    render(<People />)

    expect(screen.getByText(/unavailable/)).toBeInTheDocument()
  })
})
