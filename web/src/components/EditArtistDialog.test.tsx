import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { LibraryEntryService } from '../gen/purser/domain/v1/library_entry_pb'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { EditArtistDialog } from './EditArtistDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as PersonDialog.test.tsx.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>, entry: LibraryEntry) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onSaved = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <EditArtistDialog entry={entry} onClose={onClose} onSaved={onSaved} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onSaved }
}

const entry: LibraryEntry = {
  $typeName: 'purser.domain.v1.LibraryEntry',
  id: 'artist-1',
  contentType: 'music',
  kind: 'artist',
  name: 'REO Speedwagon',
  sortName: 'REO Speedwagon',
  overview: 'An American rock band.',
  parentId: '',
  monitored: true,
  monitorMode: MonitorMode.ALL,
  status: '',
  qualityProfileId: '',
  metadataProfileId: '',
  path: '',
  metadata: {
    artist_type: 'Group',
    aliases: ['REO'],
    founded_date: '1967',
    isni: '0000000123456789',
    official_url: 'https://reospeedwagon.com',
    genre: 'Rock',
  },
}

const barePersonEntry: LibraryEntry = {
  ...entry,
  id: 'artist-2',
  name: 'Solo Act',
  overview: '',
  metadata: {},
}

// A LibraryEntry with existing Metadata, and one with none — ADR 0004's
// "shared component test exercises more than one configuration", applied
// to this dialog's own pre-fill/initial-state branch.
describe.each([
  { label: 'a fully-populated entry', target: entry },
  { label: 'an entry with no Metadata yet', target: barePersonEntry },
])('EditArtistDialog — pre-fill ($label)', ({ target }) => {
  it('pre-fills every field UpdateLibraryEntry can change', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, target)

    expect(screen.getByLabelText('Name')).toHaveValue(target.name)
    expect(screen.getByLabelText('Overview')).toHaveValue(target.overview)
  })
})

describe('EditArtistDialog', () => {
  it('blocks submit and shows an inline error when Name is cleared', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, entry)

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: '  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Name is required.')
  })

  it('disables Save until a field is actually changed', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, entry)

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  it('submits a field mask containing only the touched top-level field', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['overview'])
          expect(req.libraryEntry?.id).toBe('artist-1')
          expect(req.libraryEntry?.overview).toBe('Updated bio.')
          return { libraryEntry: { ...entry, overview: 'Updated bio.' } }
        },
      })
    })
    const { onSaved } = renderDialog(mockTransport, entry)

    fireEvent.change(screen.getByLabelText('Overview'), { target: { value: 'Updated bio.' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ overview: 'Updated bio.' })),
    )
  })

  it('patches one metadata key while preserving untouched keys', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        updateLibraryEntry: req => {
          expect(req.updateMask?.paths).toEqual(['metadata'])
          expect(req.libraryEntry?.metadata).toEqual({
            artist_type: 'Group',
            aliases: ['REO'],
            founded_date: '1967',
            isni: '0000000199999999',
            official_url: 'https://reospeedwagon.com',
            genre: 'Rock',
          })
          return { libraryEntry: entry }
        },
      })
    })
    const { onSaved } = renderDialog(mockTransport, entry)

    fireEvent.change(screen.getByLabelText('ISNI'), { target: { value: '0000000199999999' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })

  it('shows a server error without closing when UpdateLibraryEntry fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(LibraryEntryService, {
        updateLibraryEntry: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onSaved } = renderDialog(mockTransport, entry)

    fireEvent.change(screen.getByLabelText('Overview'), { target: { value: 'Updated bio.' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't save this artist"))
    expect(onSaved).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
