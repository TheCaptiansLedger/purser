import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { usePeopleList } from '../hooks/usePeopleList'
import { usePersonImages } from '../hooks/usePersonImages'
import { People } from './People'
import { PersonSearchDialog, type PersonSearchDialogProps } from '../components/PersonSearchDialog'

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

// PersonSearchDialog's own search/filter/get-or-create behavior is
// covered by PersonSearchDialog.test.tsx — same split MusicLibrary.test.tsx
// draws around AddArtistDialog. PersonDialog (the manual path) stays
// unmocked, per this file's existing "mounts unconditionally" note above.
vi.mock('../components/PersonSearchDialog')
const mockPersonSearchDialog = vi.mocked(PersonSearchDialog)
mockPersonSearchDialog.mockImplementation(({ onClose, onAdded }: PersonSearchDialogProps) => (
  <div>
    <span>Person search dialog open</span>
    <button type="button" onClick={onClose}>
      stub-close
    </button>
    <button type="button" onClick={() => onAdded('new-person-1')}>
      stub-add
    </button>
  </div>
))

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
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof usePeopleList>
}

describe('People', () => {
  it('shows the empty-library state with an enabled Add Person action when nothing exists', () => {
    mockUsePeopleList.mockReturnValue(loaded([]))

    renderPeople()

    expect(screen.getByText('No people yet')).toBeInTheDocument()
    // The header's "Add Person" DropdownMenu trigger is the only
    // affordance — the EmptyState has no action of its own, same as
    // Music Library's equivalent empty state.
    expect(screen.getByRole('button', { name: 'Add Person' })).toBeEnabled()
  })

  it('opens PersonDialog from Add Person → Add Manually, creates the person, and navigates to their detail page', async () => {
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }]))
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        createPerson: req => ({ person: { ...req.person!, id: 'p2' } }),
      })
    })

    renderPeople(mockTransport)
    fireEvent.click(screen.getByRole('button', { name: 'Add Person' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Add Manually' }))
    expect(screen.getByRole('dialog', { name: 'Add Person' })).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Person' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.getByText('Person detail page')).toBeInTheDocument())
  })

  it('opens PersonSearchDialog from Add Person → Search MusicBrainz, and picking a result navigates', async () => {
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }]))

    renderPeople()
    expect(screen.queryByText('Person search dialog open')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Person' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Search MusicBrainz' }))
    expect(screen.getByText('Person search dialog open')).toBeInTheDocument()

    fireEvent.click(screen.getByText('stub-add'))

    await waitFor(() => expect(screen.getByText('Person detail page')).toBeInTheDocument())
  })

  it('closing PersonSearchDialog dismisses it without navigating', () => {
    mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }]))

    renderPeople()
    fireEvent.click(screen.getByRole('button', { name: 'Add Person' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Search MusicBrainz' }))
    fireEvent.click(screen.getByText('stub-close'))

    expect(screen.queryByText('Person search dialog open')).not.toBeInTheDocument()
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

  describe('bulk delete', () => {
    it('Select swaps card navigation for toggle-selection, tracked by the SelectionToolbar', () => {
      mockUsePeopleList.mockReturnValue(
        loaded([
          { id: 'p1', name: 'Jane Doe', monitored: true },
          { id: 'p2', name: 'Stevie Nicks', monitored: true },
        ]),
      )

      renderPeople()
      fireEvent.click(screen.getByRole('button', { name: 'Select' }))
      expect(screen.getByText('0 people selected')).toBeInTheDocument()

      fireEvent.click(screen.getByText('Jane Doe'))
      expect(screen.getByText('1 people selected')).toBeInTheDocument()
      expect(screen.queryByText('Person detail page')).not.toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      expect(screen.queryByText('1 people selected')).not.toBeInTheDocument()
    })

    it('confirming BulkDeleteDialog calls BulkDeletePeople with the selected ids, then refreshes the grid', async () => {
      mockUsePeopleList.mockReturnValue(loaded([{ id: 'p1', name: 'Jane Doe', monitored: true }]))

      let bulkDeleteRequest: { ids: string[]; cascade: boolean } | undefined
      const mockTransport = createRouterTransport(router => {
        router.service(PersonService, {
          getPersonDeletionImpact: () => ({
            impacts: [{ kind: 'entry_person', label: 'Credits (Library Entries)', count: 1, blocking: false }],
          }),
          bulkDeletePeople: req => {
            bulkDeleteRequest = { ids: req.ids, cascade: req.cascade }
            return {}
          },
        })
      })

      renderPeople(mockTransport)
      fireEvent.click(screen.getByRole('button', { name: 'Select' }))
      fireEvent.click(screen.getByText('Jane Doe'))
      fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

      expect(await screen.findByText('Credits (Library Entries)')).toBeInTheDocument()
      // Person never blocks a delete (person_deletion.go) — no cascade
      // checkbox to opt into.
      expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()

      fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }))

      await waitFor(() => expect(bulkDeleteRequest).toEqual({ ids: ['p1'], cascade: false }))
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      expect(screen.queryByText('1 people selected')).not.toBeInTheDocument()
    })
  })
})
