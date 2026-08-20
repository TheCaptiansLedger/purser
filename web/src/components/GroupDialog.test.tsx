import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ComponentProps } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { type Group, GroupService } from '../gen/purser/domain/v1/group_pb'
import { GroupDialog } from './GroupDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as PersonDialog.test.tsx,
// this component's closest existing analog (one dialog, two modes).
function renderDialog(
  mockTransport: ReturnType<typeof createRouterTransport>,
  props: Partial<ComponentProps<typeof GroupDialog>> & Pick<ComponentProps<typeof GroupDialog>, 'mode'>,
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onSaved = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <GroupDialog onClose={onClose} onSaved={onSaved} {...props} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onSaved }
}

const existingGroup: Group = {
  $typeName: 'purser.domain.v1.Group',
  id: 'group-1',
  libraryEntryId: 'artist-1',
  title: 'Live Aid Bootleg',
  sortName: 'Live Aid Bootleg',
  number: '',
  year: 1985,
  overview: 'A bootleg.',
  monitored: true,
  monitorMode: MonitorMode.ALL,
} as Group

// Create mode — the first content-type/prop configuration (ADR 0004: a
// shared component's test exercises more than one).
describe('GroupDialog — create', () => {
  it('submits a fully-formed Group with no ExternalID call, and reports it saved', async () => {
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
    const { onClose, onSaved } = renderDialog(mockTransport, { mode: 'create', artistId: 'artist-1' })

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ id: 'group-2' })))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('blocks submit and shows an inline error when Title is blank', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, { mode: 'create', artistId: 'artist-1' })

    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Title is required.')
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
    const { onSaved } = renderDialog(mockTransport, { mode: 'create', artistId: 'artist-1' })

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })

  it('shows an inline error and stays open when CreateGroup fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        createGroup: () => {
          throw new Error('unavailable')
        },
      })
    })
    const { onClose, onSaved } = renderDialog(mockTransport, { mode: 'create', artistId: 'artist-1' })

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Live Aid Bootleg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't save this album"))
    expect(onSaved).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})

// Edit mode — the second content-type/prop configuration, exercising
// pre-fill and field-mask diffing, and the monitor-field exclusion #681
// requires (create mode has none of this).
describe('GroupDialog — edit', () => {
  it('pre-fills every field UpdateGroup can change, with no Monitored toggle', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, { mode: 'edit', group: existingGroup })

    expect(screen.getByLabelText('Title')).toHaveValue('Live Aid Bootleg')
    expect(screen.getByLabelText('Sort title')).toHaveValue('Live Aid Bootleg')
    expect(screen.getByLabelText('Year')).toHaveValue(1985)
    expect(screen.getByLabelText('Overview')).toHaveValue('A bootleg.')
    expect(screen.queryByText('Monitored')).not.toBeInTheDocument()
  })

  it('submits a field mask containing only the touched field, never monitored', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(GroupService, {
        updateGroup: req => {
          expect(req.updateMask?.paths).toEqual(['year'])
          expect(req.group?.id).toBe('group-1')
          expect(req.group?.year).toBe(1986)
          return { group: { ...existingGroup, year: 1986 } }
        },
      })
    })
    const { onSaved } = renderDialog(mockTransport, { mode: 'edit', group: existingGroup })

    fireEvent.change(screen.getByLabelText('Year'), { target: { value: '1986' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ year: 1986 })))
  })

  it('disables Save until a field is actually changed', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, { mode: 'edit', group: existingGroup })

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })
})
