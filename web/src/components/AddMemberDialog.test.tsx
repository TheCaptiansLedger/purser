import { ConnectError, Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { AddMemberDialog } from './AddMemberDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as PersonDialog.test.tsx.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <AddMemberDialog libraryEntryId="artist-1" onClose={onClose} onAdded={onAdded} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

// Search-existing-person path — the first sub-flow configuration.
describe('AddMemberDialog — search existing', () => {
  it('finds a person, then submits role/era via CreateEntryPerson', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        listPeople: req => {
          expect(req.name).toBe('Stevie')
          return { people: [{ id: 'person-1', name: 'Stevie Nicks' }] }
        },
      })
      router.service(EntryPersonService, {
        createEntryPerson: req => {
          expect(req.entryPerson?.libraryEntryId).toBe('artist-1')
          expect(req.entryPerson?.personId).toBe('person-1')
          expect(req.entryPerson?.role).toBe('vocalist')
          return { entryPerson: req.entryPerson }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Search existing person' }))
    fireEvent.change(screen.getByLabelText('Search people'), { target: { value: 'Stevie' } })

    fireEvent.click(await screen.findByRole('button', { name: 'Stevie Nicks' }))
    fireEvent.change(screen.getByLabelText('Role'), { target: { value: 'vocalist' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalled())
  })

  it('blocks submit and shows an inline error when Role is blank', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, { listPeople: () => ({ people: [{ id: 'person-1', name: 'Stevie Nicks' }] }) })
    })
    renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Search existing person' }))
    fireEvent.change(screen.getByLabelText('Search people'), { target: { value: 'Stevie' } })
    fireEvent.click(await screen.findByRole('button', { name: 'Stevie Nicks' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Role is required')
  })

  it('surfaces a CreateEntryPerson AlreadyExists conflict as a plain inline error', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, { listPeople: () => ({ people: [{ id: 'person-1', name: 'Stevie Nicks' }] }) })
      router.service(EntryPersonService, {
        createEntryPerson: () => {
          throw new ConnectError('already linked', Code.AlreadyExists)
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Search existing person' }))
    fireEvent.change(screen.getByLabelText('Search people'), { target: { value: 'Stevie' } })
    fireEvent.click(await screen.findByRole('button', { name: 'Stevie Nicks' }))
    fireEvent.change(screen.getByLabelText('Role'), { target: { value: 'vocalist' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('Stevie Nicks already holds the "vocalist" role.'),
    )
    expect(onAdded).not.toHaveBeenCalled()
  })
})

// Create-new-person path — the second sub-flow configuration, reusing
// PersonDialog as a sequential step rather than searching.
describe('AddMemberDialog — create new', () => {
  it('creates a Person via the embedded PersonDialog, then submits role/era via CreateEntryPerson', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        createPerson: req => ({ person: { ...req.person!, id: 'person-2' } }),
      })
      router.service(EntryPersonService, {
        createEntryPerson: req => {
          expect(req.entryPerson?.personId).toBe('person-2')
          expect(req.entryPerson?.role).toBe('guitarist')
          return { entryPerson: req.entryPerson }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Create new person' }))
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Member' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    fireEvent.change(await screen.findByLabelText('Role'), { target: { value: 'guitarist' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalled())
  })
})
