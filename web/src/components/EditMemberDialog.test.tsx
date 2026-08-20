import { ConnectError, Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { describe, expect, it, vi } from 'vitest'
import type { ComponentProps } from 'react'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { EditMemberDialog } from './EditMemberDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as PersonDialog.test.tsx.
function renderDialog(
  mockTransport: ReturnType<typeof createRouterTransport>,
  props: Partial<ComponentProps<typeof EditMemberDialog>> = {},
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onSaved = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <EditMemberDialog
          libraryEntryId="artist-1"
          personId="person-1"
          personName="Stevie Nicks"
          role="vocalist"
          startDate={timestampFromDate(new Date('1975-01-01'))}
          onClose={onClose}
          onSaved={onSaved}
          {...props}
        />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onSaved }
}

// Era-only change — role untouched, goes through UpdateEntryPerson with a
// minimal field mask (the first configuration).
describe('EditMemberDialog — era change, role unchanged', () => {
  it('submits UpdateEntryPerson with a field mask of only the changed date', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        updateEntryPerson: req => {
          expect(req.entryPerson?.role).toBe('vocalist')
          expect(req.updateMask?.paths).toEqual(['end_date'])
          return { entryPerson: req.entryPerson }
        },
      })
    })
    const { onSaved } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('End date'), { target: { value: '1987-01-01' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })

  it('disables Save until something is actually changed', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport)

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })
})

// Role change — UpdateEntryPerson can't rename the identity key, so this
// path is CreateEntryPerson(new role) then DeleteEntryPerson(old role) —
// the second configuration, exercising behavior the era-only path has
// none of.
describe('EditMemberDialog — role change', () => {
  it('creates the new role then deletes the old one', async () => {
    const created: unknown[] = []
    let deletedRole: string | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        createEntryPerson: req => {
          created.push(req.entryPerson)
          return { entryPerson: req.entryPerson }
        },
        deleteEntryPerson: req => {
          deletedRole = req.role
          return {}
        },
      })
    })
    const { onSaved } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Role'), { target: { value: 'backing vocalist' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
    expect(created).toEqual([expect.objectContaining({ role: 'backing vocalist', personId: 'person-1' })])
    expect(deletedRole).toBe('vocalist')
  })

  it('surfaces an AlreadyExists conflict inline and leaves the old role untouched', async () => {
    let deleteCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(EntryPersonService, {
        createEntryPerson: () => {
          throw new ConnectError('already linked', Code.AlreadyExists)
        },
        deleteEntryPerson: () => {
          deleteCalled = true
          return {}
        },
      })
    })
    const { onSaved } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Role'), { target: { value: 'guitarist' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('Stevie Nicks already holds the "guitarist" role.'),
    )
    expect(onSaved).not.toHaveBeenCalled()
    expect(deleteCalled).toBe(false)
  })
})
