import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { GroupService } from '../gen/purser/domain/v1/group_pb'
import { ManualAlbumDialog } from './ManualAlbumDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as PersonDialog.test.tsx,
// this component's closest existing analog (manual-entry-only dialog).
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <ManualAlbumDialog artistId="artist-1" onClose={onClose} onAdded={onAdded} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

describe('ManualAlbumDialog', () => {
  it('submits a fully-formed Group with no ExternalID call, and reports it added', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        createGroup: req => {
          expect(req.group?.libraryEntryId).toBe('artist-1')
          expect(req.group?.title).toBe('Live Aid Bootleg')
          expect(req.group?.monitored).toBe(true)
          expect(req.group?.monitorMode).toBe(MonitorMode.ALL)
          return { group: { ...req.group!, id: 'group-2' } }
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'group-2' })))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('blocks submit and shows an inline error when Title is blank', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Title is required.')
  })

  it('sends Year as the typed number when filled in', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        createGroup: req => {
          expect(req.group?.year).toBe(1985)
          return { group: { ...req.group!, id: 'group-2' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.change(screen.getByLabelText('Year'), { target: { value: '1985' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalled())
  })

  it('sends Year as 0 when left blank', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        createGroup: req => {
          expect(req.group?.year).toBe(0)
          return { group: { ...req.group!, id: 'group-2' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalled())
  })

  it('shows an inline error and stays open when CreateGroup fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        createGroup: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't add this album"))
    expect(onAdded).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
