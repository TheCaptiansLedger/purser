import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ComponentProps } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import { ManualArtistDialog } from './ManualArtistDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern GroupDialog.test.tsx/
// ManualEditionDialog.test.tsx already use.
function renderDialog(
  mockTransport: ReturnType<typeof createRouterTransport>,
  props: Partial<ComponentProps<typeof ManualArtistDialog>> = {},
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <ManualArtistDialog onClose={onClose} onAdded={onAdded} {...props} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

function fillRequired(name = 'The Unsigned') {
  fireEvent.change(screen.getByLabelText('Name'), { target: { value: name } })
  fireEvent.change(screen.getByLabelText('Artist type'), { target: { value: 'Group' } })
}

describe('ManualArtistDialog', () => {
  it('blocks submit and shows an inline error when Name is blank', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Artist type'), { target: { value: 'Group' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(screen.getByText('Name is required.')).toBeInTheDocument()
  })

  it('blocks submit and shows an inline error when Artist type is unselected', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'The Unsigned' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(screen.getByText('Artist type is required.')).toBeInTheDocument()
  })

  // Group branch — the first artist_type/prop configuration (ADR 0011's
  // acceptance criteria for this issue: both branches get exercised).
  it('shows Founded/Dissolved fields and submits founded_date/dissolved_date metadata for artist_type=Group', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        createLibraryEntry: req => {
          expect(req.libraryEntry?.contentType).toBe('music')
          expect(req.libraryEntry?.kind).toBe('artist')
          expect(req.libraryEntry?.name).toBe('The Unsigned')
          expect(req.libraryEntry?.monitored).toBe(true)
          expect(req.libraryEntry?.monitorMode).toBe(MonitorMode.ALL)
          expect(req.libraryEntry?.metadata).toEqual({
            artist_type: 'Group',
            country: 'US',
            founded_date: '1998',
            dissolved_date: '2010',
          })
          return { libraryEntry: { ...req.libraryEntry!, id: 'artist-1' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fillRequired()
    expect(screen.getByLabelText('Founded')).toBeInTheDocument()
    expect(screen.getByLabelText('Dissolved')).toBeInTheDocument()
    expect(screen.queryByLabelText('Born')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Died')).not.toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Country'), { target: { value: 'US' } })
    fireEvent.change(screen.getByLabelText('Founded'), { target: { value: '1998' } })
    fireEvent.change(screen.getByLabelText('Dissolved'), { target: { value: '2010' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'artist-1' })))
  })

  // Person branch — the second artist_type/prop configuration.
  it('shows Born/Died fields and submits born_date/died_date metadata for artist_type=Person', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        createLibraryEntry: req => {
          expect(req.libraryEntry?.metadata).toEqual({
            artist_type: 'Person',
            born_date: '1970',
            died_date: '2020',
          })
          return { libraryEntry: { ...req.libraryEntry!, id: 'artist-2' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Solo Act' } })
    fireEvent.change(screen.getByLabelText('Artist type'), { target: { value: 'Person' } })
    expect(screen.getByLabelText('Born')).toBeInTheDocument()
    expect(screen.getByLabelText('Died')).toBeInTheDocument()
    expect(screen.queryByLabelText('Founded')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Dissolved')).not.toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Born'), { target: { value: '1970' } })
    fireEvent.change(screen.getByLabelText('Died'), { target: { value: '2020' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 'artist-2' })))
  })

  it('omits blank optional metadata fields entirely, rather than sending empty strings', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        createLibraryEntry: req => {
          expect(req.libraryEntry?.metadata).toEqual({ artist_type: 'Group' })
          return { libraryEntry: { ...req.libraryEntry!, id: 'artist-3' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fillRequired()
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onAdded).toHaveBeenCalled())
  })

  it('shows an inline error and stays open when CreateLibraryEntry fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        createLibraryEntry: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fillRequired()
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't add this artist"))
    expect(onAdded).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('calls onClose from Cancel without submitting', () => {
    const mockTransport = createRouterTransport(() => {})
    const { onClose } = renderDialog(mockTransport)

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onClose).toHaveBeenCalled()
  })
})
