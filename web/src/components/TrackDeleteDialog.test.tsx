import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ItemService } from '../gen/purser/domain/v1/item_pb'
import { TrackDeleteDialog } from './TrackDeleteDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onDeleted = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <TrackDeleteDialog trackId="item-1" trackTitle="Dreams" onClose={onClose} onDeleted={onDeleted} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onDeleted }
}

describe('TrackDeleteDialog', () => {
  it('shows a real referrer from GetItemDeletionImpact (a track with a MediaFile attached)', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        getItemDeletionImpact: req => {
          expect(req.id).toBe('item-1')
          return {
            impacts: [
              { kind: 'media_file', label: 'Media Files', count: 1, blocking: false },
              { kind: 'external_id', label: 'External IDs', count: 0, blocking: false },
            ],
          }
        },
      })
    })
    renderDialog(mockTransport)

    expect(await screen.findByText('Media Files')).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.queryByText('External IDs')).not.toBeInTheDocument()
  })

  it('shows nothing-references-this when every impact row is zero', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        getItemDeletionImpact: () => ({
          impacts: [{ kind: 'media_file', label: 'Media Files', count: 0, blocking: false }],
        }),
      })
    })
    renderDialog(mockTransport)

    expect(await screen.findByText('Nothing else references this track.')).toBeInTheDocument()
  })

  it('confirming Delete calls DeleteItem and reports onDeleted', async () => {
    let deletedId: string | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        getItemDeletionImpact: () => ({ impacts: [] }),
        deleteItem: req => {
          deletedId = req.id
          return {}
        },
      })
    })
    const { onDeleted } = renderDialog(mockTransport)

    await screen.findByText('Nothing else references this track.')
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(onDeleted).toHaveBeenCalled())
    expect(deletedId).toBe('item-1')
  })

  it('shows an inline error and stays open when DeleteItem fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(ItemService, {
        getItemDeletionImpact: () => ({ impacts: [] }),
        deleteItem: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onDeleted } = renderDialog(mockTransport)

    await screen.findByText('Nothing else references this track.')
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't delete this track"))
    expect(onDeleted).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
