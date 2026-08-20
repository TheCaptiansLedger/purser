import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { PersonDeleteDialog } from './PersonDeleteDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// TrackDeleteDialog.test.tsx, the single-delete precedent this mirrors.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onDeleted = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <PersonDeleteDialog personId="person-1" personName="Stevie Nicks" onClose={onClose} onDeleted={onDeleted} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onDeleted }
}

describe('PersonDeleteDialog', () => {
  it('shows a real referrer from GetPersonDeletionImpact', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPersonDeletionImpact: req => {
          expect(req.id).toBe('person-1')
          return {
            impacts: [
              { kind: 'entry_person', label: 'Credits (Library Entries)', count: 2, blocking: false },
              { kind: 'image', label: 'Images', count: 0, blocking: false },
            ],
          }
        },
      })
    })
    renderDialog(mockTransport)

    expect(await screen.findByText('Credits (Library Entries)')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.queryByText('Images')).not.toBeInTheDocument()
  })

  it('shows nothing-references-this when every impact row is zero', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPersonDeletionImpact: () => ({
          impacts: [{ kind: 'image', label: 'Images', count: 0, blocking: false }],
        }),
      })
    })
    renderDialog(mockTransport)

    expect(await screen.findByText('Nothing else references this person.')).toBeInTheDocument()
  })

  it('confirming Delete calls DeletePerson with cascade=false and reports onDeleted', async () => {
    let deletedId: string | undefined
    let gotCascade: boolean | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPersonDeletionImpact: () => ({ impacts: [] }),
        deletePerson: req => {
          deletedId = req.id
          gotCascade = req.cascade
          return {}
        },
      })
    })
    const { onDeleted } = renderDialog(mockTransport)

    await screen.findByText('Nothing else references this person.')
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(onDeleted).toHaveBeenCalled())
    expect(deletedId).toBe('person-1')
    expect(gotCascade).toBe(false)
  })

  it('shows an inline error and stays open when DeletePerson fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPersonDeletionImpact: () => ({ impacts: [] }),
        deletePerson: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onDeleted } = renderDialog(mockTransport)

    await screen.findByText('Nothing else references this person.')
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't delete this person"))
    expect(onDeleted).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
