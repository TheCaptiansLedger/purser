import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MusicReleaseService } from '../gen/purser/music/v1/release_pb'
import { AddTrackDialog } from './AddTrackDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// ManualEditionDialog.test.tsx, this component's closest existing analog.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <AddTrackDialog releaseId="release-1" onClose={onClose} onAdded={onAdded} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

describe('AddTrackDialog', () => {
  it('submits title/number/medium/runtime with no mbid — a hand-entered track', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        createMusicReleaseTrack: req => {
          expect(req.releaseId).toBe('release-1')
          expect(req.title).toBe('Come On Eileen')
          expect(req.number).toBe('3')
          expect(req.mediumNumber).toBe(1)
          expect(req.runtimeSeconds).toBe(258)
          expect(req.mbid).toBe('')
          return { track: { id: 'item-1', title: req.title, sequence: req.number } }
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Come On Eileen' } })
    fireEvent.change(screen.getByLabelText(/^Number/), { target: { value: '3' } })
    fireEvent.change(screen.getByLabelText(/^Runtime/), { target: { value: '258' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'item-1' })))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('defaults medium number to 1 when left unchanged', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        createMusicReleaseTrack: req => {
          expect(req.mediumNumber).toBe(1)
          return { track: { id: 'item-1', title: req.title } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Track' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalled())
  })

  it('blocks submit and shows an inline error when Title is blank', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Title is required.')
  })

  it('shows an inline error and stays open when CreateMusicReleaseTrack fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        createMusicReleaseTrack: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Track' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't add this track"))
    expect(onAdded).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
