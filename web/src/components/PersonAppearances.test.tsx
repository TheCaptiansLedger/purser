import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { ItemPersonService } from '../gen/purser/domain/v1/item_person_pb'
import { ItemService } from '../gen/purser/domain/v1/item_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { PersonAppearances } from './PersonAppearances'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend.
function renderAppearances(mockTransport: ReturnType<typeof createRouterTransport>, personId = 'person-1') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <PersonAppearances personId={personId} />
      </QueryClientProvider>
    </TransportProvider>,
  )
}

function noEntryPeople() {
  return { entryPeople: [] as never[], nextPageToken: '' }
}

function noItemPeople() {
  return { itemPeople: [] as never[], nextPageToken: '' }
}

describe('PersonAppearances', () => {
  it('renders "No known appearances yet" when both lists are empty', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, { listEntryPeople: noEntryPeople })
      router.service(ItemPersonService, { listItemPeople: noItemPeople })
    })

    renderAppearances(mockTransport)

    await waitFor(() => expect(screen.getByText('No known appearances yet.')).toBeInTheDocument())
  })

  it('renders correctly when only the LibraryEntry list has rows', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        listEntryPeople: () => ({
          entryPeople: [{ libraryEntryId: 'entry-1', personId: 'person-1', role: 'Director' }],
          nextPageToken: '',
        }),
      })
      router.service(ItemPersonService, { listItemPeople: noItemPeople })
      router.service(LibraryEntryService, {
        getLibraryEntry: () => ({ libraryEntry: { id: 'entry-1', name: 'Rumours' } }),
      })
    })

    renderAppearances(mockTransport)

    await waitFor(() => expect(screen.getByText('Rumours')).toBeInTheDocument())
    expect(screen.getByText('Director')).toBeInTheDocument()
    expect(screen.queryByText('No known appearances yet.')).not.toBeInTheDocument()
  })

  it('renders correctly when only the Item list has rows', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, { listEntryPeople: noEntryPeople })
      router.service(ItemPersonService, {
        listItemPeople: () => ({
          itemPeople: [{ itemId: 'item-1', personId: 'person-1', role: 'Guest Vocalist' }],
          nextPageToken: '',
        }),
      })
      router.service(ItemService, { getItem: () => ({ item: { id: 'item-1', title: 'Dreams' } }) })
    })

    renderAppearances(mockTransport)

    await waitFor(() => expect(screen.getByText('Dreams')).toBeInTheDocument())
    expect(screen.getByText('Guest Vocalist')).toBeInTheDocument()
    expect(screen.queryByText('No known appearances yet.')).not.toBeInTheDocument()
  })

  it('renders rows from both lists together', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        listEntryPeople: () => ({
          entryPeople: [{ libraryEntryId: 'entry-1', personId: 'person-1', role: 'Director' }],
          nextPageToken: '',
        }),
      })
      router.service(ItemPersonService, {
        listItemPeople: () => ({
          itemPeople: [{ itemId: 'item-1', personId: 'person-1', role: 'Guest Vocalist' }],
          nextPageToken: '',
        }),
      })
      router.service(LibraryEntryService, {
        getLibraryEntry: () => ({ libraryEntry: { id: 'entry-1', name: 'Rumours' } }),
      })
      router.service(ItemService, { getItem: () => ({ item: { id: 'item-1', title: 'Dreams' } }) })
    })

    renderAppearances(mockTransport)

    await waitFor(() => expect(screen.getByText('Rumours')).toBeInTheDocument())
    expect(screen.getByText('Dreams')).toBeInTheDocument()
    expect(screen.getByText('Director')).toBeInTheDocument()
    expect(screen.getByText('Guest Vocalist')).toBeInTheDocument()
  })

  it('shows an actionable error when a list call fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        listEntryPeople: () => {
          throw new Error('unavailable')
        },
      })
      router.service(ItemPersonService, { listItemPeople: noItemPeople })
    })

    renderAppearances(mockTransport)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load appearances"))
  })
})
