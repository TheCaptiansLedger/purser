import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { RemoveMemberDialog } from './RemoveMemberDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as TrackDeleteDialog's
// own precedent for a plain (no deletion-impact) confirm.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onRemoved = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <RemoveMemberDialog
          libraryEntryId="artist-1"
          personId="person-1"
          personName="Stevie Nicks"
          role="vocalist"
          onClose={onClose}
          onRemoved={onRemoved}
        />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onRemoved }
}

// Confirm/success path — the first configuration.
describe('RemoveMemberDialog — confirm', () => {
  it('names the person and role in the confirm text, with no deletion-impact query', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport)

    expect(screen.getByText(/Remove Stevie Nicks from the "vocalist" role/)).toBeInTheDocument()
  })

  it('calls DeleteEntryPerson with the row identity and reports removed', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        deleteEntryPerson: req => {
          expect(req.libraryEntryId).toBe('artist-1')
          expect(req.personId).toBe('person-1')
          expect(req.role).toBe('vocalist')
          return {}
        },
      })
    })
    const { onRemoved } = renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))

    await waitFor(() => expect(onRemoved).toHaveBeenCalled())
  })
})

// Error path — the second configuration, exercising the failure branch.
describe('RemoveMemberDialog — error', () => {
  it('shows a server error without closing when DeleteEntryPerson fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        deleteEntryPerson: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onRemoved } = renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't remove this member"))
    expect(onRemoved).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
