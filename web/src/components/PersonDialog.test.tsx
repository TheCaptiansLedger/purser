import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ComponentProps } from 'react'
import { Gender, type Person, PersonService } from '../gen/purser/domain/v1/person_pb'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { PersonDialog } from './PersonDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// ChooseArtworkDialog.test.tsx.
function renderDialog(
  mockTransport: ReturnType<typeof createRouterTransport>,
  props: Partial<ComponentProps<typeof PersonDialog>> & Pick<ComponentProps<typeof PersonDialog>, 'mode'>,
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onSaved = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <PersonDialog onClose={onClose} onSaved={onSaved} {...props} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onSaved }
}

const existingPerson: Person = {
  $typeName: 'purser.domain.v1.Person',
  id: 'person-1',
  name: 'Jamie Rivers',
  sortName: 'Rivers, Jamie',
  aliases: ['JR'],
  gender: Gender.NON_BINARY,
  pronouns: 'they/them',
  nationality: 'US',
  overview: 'A performer.',
  monitored: true,
  monitorMode: MonitorMode.ALL,
} as Person

// Create mode — the first content-type/prop configuration (ADR 0004: a
// shared component's test exercises more than one).
describe('PersonDialog — create', () => {
  it('submits a fully-formed Person and reports it saved', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        createPerson: req => {
          expect(req.person?.name).toBe('New Person')
          expect(req.person?.gender).toBe(Gender.UNKNOWN)
          expect(req.person?.monitorMode).toBe(MonitorMode.ALL)
          expect(req.person?.monitored).toBe(true)
          return { person: { ...req.person!, id: 'person-2' } }
        },
      })
    })
    const { onClose, onSaved } = renderDialog(mockTransport, { mode: 'create' })

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Person' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ id: 'person-2' })))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('blocks submit and shows an inline error when Name is blank', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, { mode: 'create' })

    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Name is required')
  })

  it('shows a server error without closing when CreatePerson fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        createPerson: () => {
          throw new Error('name already taken')
        },
      })
    })
    const { onClose, onSaved } = renderDialog(mockTransport, { mode: 'create' })

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Person' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't save this person"))
    expect(onSaved).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})

// Edit mode — the second content-type/prop configuration, exercising
// pre-fill and field-mask diffing, the behavior create mode has none of.
describe('PersonDialog — edit', () => {
  it('pre-fills every field UpdatePerson can change', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, { mode: 'edit', person: existingPerson })

    expect(screen.getByLabelText('Name')).toHaveValue('Jamie Rivers')
    expect(screen.getByLabelText('Sort name')).toHaveValue('Rivers, Jamie')
    expect(screen.getByText('JR')).toBeInTheDocument()
    expect(screen.getByLabelText('Pronouns')).toHaveValue('they/them')
    expect(screen.getByLabelText('Nationality')).toHaveValue('US')
    expect(screen.getByLabelText('Overview')).toHaveValue('A performer.')
  })

  it('submits a field mask containing only the touched field', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        updatePerson: req => {
          expect(req.updateMask?.paths).toEqual(['pronouns'])
          expect(req.person?.id).toBe('person-1')
          expect(req.person?.pronouns).toBe('she/her')
          return { person: { ...existingPerson, pronouns: 'she/her' } }
        },
      })
    })
    const { onSaved } = renderDialog(mockTransport, { mode: 'edit', person: existingPerson })

    fireEvent.change(screen.getByLabelText('Pronouns'), { target: { value: 'she/her' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ pronouns: 'she/her' })),
    )
  })

  it('disables Save until a field is actually changed', () => {
    const mockTransport = createRouterTransport(() => {})
    renderDialog(mockTransport, { mode: 'edit', person: existingPerson })

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })
})
