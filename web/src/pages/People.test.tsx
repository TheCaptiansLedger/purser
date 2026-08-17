import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { usePeopleList } from '../hooks/usePeopleList'
import { usePersonImages } from '../hooks/usePersonImages'
import { People } from './People'

// People navigates via useNavigate(), which throws outside a Router
// context — every render needs one. A /people/:id route is present so a
// navigation assertion can check the resulting location's rendered output
// instead of reaching into the router's internals. TransportProvider/
// QueryClientProvider wrap every render (not just the Add-Person tests)
// since PersonDialog (#663) mounts unconditionally once opened and its
// mutation hooks require both contexts to exist.
function renderPeople(mockTransport: ReturnType<typeof createRouterTransport> = createRouterTransport(() => {})) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/people']}>
          <Routes>
            <Route path="/people" element={<People />} />
            <Route path="/people/:id" element={<div>Person detail page</div>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </TransportProvider>,
  )
}

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
  it('shows the empty-library state with an enabled Add Person action when nothing exists', () => {
    mockUsePeopleList.mockReturnValue(loaded([]))

    renderPeople()

    expect(screen.getByText('No people yet')).toBeInTheDocument()
    // Two "Add Person" affordances exist on an empty library: the header
    // button (always present) and the EmptyState action — both open the
    // same PersonDialog.
    for (const button of screen.getAllByRole('button', { name: 'Add Person' })) {
      expect(button).toBeEnabled()
    }
  })

  it('opens PersonDialog from the header button, creates the person, and navigates to their detail page', async () => {
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }]))
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        createPerson: req => ({ person: { ...req.person!, id: 'p2' } }),
      })
    })

    renderPeople(mockTransport)
    fireEvent.click(screen.getByRole('button', { name: 'Add Person' }))
    expect(screen.getByRole('dialog', { name: 'Add Person' })).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Person' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.getByText('Person detail page')).toBeInTheDocument())
  })

  it('renders a PersonCard grid for a loaded page', () => {
    mockUsePeopleList.mockReturnValue(
      loaded([
        { id: 'p1', name: 'Jane Doe', monitored: true },
        { id: 'p2', name: 'Stevie Nicks', monitored: false },
      ]),
    )

    renderPeople()

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

    renderPeople()
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
    expect(screen.queryByText('Stevie Nicks')).not.toBeInTheDocument()
  })

  it('shows the zero-results EmptyState when Monitored only filters out every loaded person', () => {
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: false }]))

    renderPeople()
    fireEvent.click(screen.getByRole('switch', { name: 'Monitored only' }))

    expect(screen.getByText('No matches')).toBeInTheDocument()
    // A zero-results EmptyState has no action of its own — only the
    // always-present header button remains.
    expect(screen.getAllByRole('button', { name: 'Add Person' })).toHaveLength(1)
  })

  it('calls fetchNextPage from Load more without unmounting the existing grid', () => {
    const listResult = loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }], true)
    mockUsePeopleList.mockReturnValue(listResult)

    renderPeople()
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))

    expect(listResult.fetchNextPage).toHaveBeenCalledOnce()
    expect(screen.getByText('Jane Doe')).toBeInTheDocument()
  })

  it('shows nothing while the first page is pending', () => {
    mockUsePeopleList.mockReturnValue(pending())

    renderPeople()

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

    renderPeople()

    expect(screen.getByText(/unavailable/)).toBeInTheDocument()
  })

  it('navigates to the Person detail page when a card is clicked', () => {
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }]))

    renderPeople()
    fireEvent.click(screen.getByRole('link', { name: 'View Jane Doe' }))

    expect(screen.getByText('Person detail page')).toBeInTheDocument()
  })

  it('opens the lightbox instead of navigating when the photo itself is clicked', () => {
    mockUsePersonImages.mockReturnValueOnce({ p1: 'img-1' })
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }]))

    renderPeople()
    fireEvent.click(screen.getByRole('button', { name: "View Jane Doe's photo" }))

    expect(screen.getByRole('dialog', { name: 'Jane Doe' })).toBeInTheDocument()
    expect(screen.queryByText('Person detail page')).not.toBeInTheDocument()
  })
})
