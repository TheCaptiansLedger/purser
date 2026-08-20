import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MusicReleaseService, ReleaseStatus } from '../gen/purser/music/v1/release_pb'
import { ManualEditionDialog } from './ManualEditionDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// GroupDialog.test.tsx, this component's closest existing analog.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <ManualEditionDialog groupId="group-1" libraryEntryId="entry-1" onClose={onClose} onAdded={onAdded} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

describe('ManualEditionDialog', () => {
  it('submits a fully-formed Release with no mbid field, always stub/not-default', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        createMusicRelease: req => {
          expect(req.musicRelease?.groupId).toBe('group-1')
          expect(req.musicRelease?.libraryEntryId).toBe('entry-1')
          expect(req.musicRelease?.title).toBe('Live Bootleg')
          expect(req.musicRelease?.mbid).toBe('')
          expect(req.musicRelease?.status).toBe(ReleaseStatus.STUB)
          expect(req.musicRelease?.isDefault).toBe(false)
          return { musicRelease: { ...req.musicRelease!, id: 'release-2' } }
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'release-2' })))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('sends Medium/Track count as the typed numbers when filled in', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        createMusicRelease: req => {
          expect(req.musicRelease?.mediumCount).toBe(2)
          expect(req.musicRelease?.trackCount).toBe(20)
          return { musicRelease: { ...req.musicRelease!, id: 'release-2' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Bootleg' } })
    fireEvent.change(screen.getByLabelText('Medium count'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('Track count'), { target: { value: '20' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalled())
  })

  it('blocks submit and shows an inline error when Title is blank', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Title is required.')
  })

  it('shows an inline error and stays open when CreateMusicRelease fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicReleaseService, {
        createMusicRelease: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't add this edition"))
    expect(onAdded).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
