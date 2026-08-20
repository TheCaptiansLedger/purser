import { ConnectError, Code } from '@connectrpc/connect'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { Search, SearchX } from 'lucide-react'
import { useEffect, useState } from 'react'
import { createEntryPerson } from '../gen/purser/domain/v1/entry_person-EntryPersonService_connectquery'
import { listPeople } from '../gen/purser/domain/v1/person-PersonService_connectquery'
import { EmptyState } from './EmptyState'
import { Modal } from './Modal'
import { PersonDialog } from './PersonDialog'
import { TextInput } from './TextInput'

const MIN_QUERY_LENGTH = 2
const DEBOUNCE_MS = 300
const SEARCH_PAGE_SIZE = 20

// useDebouncedValue — same local helper AddArtistDialog's own search box
// uses; not worth extracting for a second, unrelated consumer.
function useDebouncedValue(value: string, delayMs: number): string {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])
  return debounced
}

type Step = 'choose' | 'search' | 'create' | 'role'

interface ChosenPerson {
  id: string
  name: string
}

export interface AddMemberDialogProps {
  libraryEntryId: string
  onClose: () => void
  onAdded: () => void
}

// AddMemberDialog is #724's "Add member" flow — the same "search existing
// or create new" shape "Add Artist"/"Add Person" already use, just over
// PersonService.ListPeople instead of a MusicBrainz search, and finishing
// with a role/era step CreateEntryPerson needs that neither of those flows
// does. One step at a time, not nested: the 'create' step renders
// PersonDialog directly in place of this dialog's own Modal (sequential,
// not simultaneous), reusing #663's create form unmodified rather than
// duplicating it inline, per this issue's own scope decision.
export function AddMemberDialog({ libraryEntryId, onClose, onAdded }: AddMemberDialogProps) {
  const [step, setStep] = useState<Step>('choose')
  const [person, setPerson] = useState<ChosenPerson | undefined>(undefined)
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebouncedValue(query, DEBOUNCE_MS).trim()
  const [role, setRole] = useState('')
  const [roleTouched, setRoleTouched] = useState(false)
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')

  const searchQuery = useQuery(
    listPeople,
    { pageSize: SEARCH_PAGE_SIZE, pageToken: '', name: debouncedQuery },
    { enabled: step === 'search' && debouncedQuery.length >= MIN_QUERY_LENGTH },
  )
  const results = searchQuery.data?.people ?? []

  const createEntryPersonMutation = useMutation(createEntryPerson)

  function selectPerson(chosen: ChosenPerson) {
    setPerson(chosen)
    setStep('role')
  }

  const trimmedRole = role.trim()
  const roleInvalid = trimmedRole === ''

  function handleSubmitRole(event: React.FormEvent) {
    event.preventDefault()
    setRoleTouched(true)
    if (roleInvalid || !person) return

    createEntryPersonMutation.mutate(
      {
        entryPerson: {
          libraryEntryId,
          personId: person.id,
          role: trimmedRole,
          startDate: startDate ? timestampFromDate(new Date(startDate)) : undefined,
          endDate: endDate ? timestampFromDate(new Date(endDate)) : undefined,
        },
      },
      { onSuccess: onAdded },
    )
  }

  if (step === 'create') {
    return (
      <PersonDialog
        mode="create"
        onClose={() => setStep('choose')}
        onSaved={created => selectPerson({ id: created.id, name: created.name })}
      />
    )
  }

  if (step === 'role' && person) {
    const conflict = createEntryPersonMutation.isError && ConnectError.from(createEntryPersonMutation.error).code === Code.AlreadyExists

    return (
      <Modal title={`Add ${person.name} as a member`} onClose={onClose}>
        <form onSubmit={handleSubmitRole} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1">
            <TextInput label="Role" value={role} onChange={v => setRole(String(v))} />
            {roleTouched && roleInvalid && (
              <span className="text-label text-status-failure" role="alert">
                Role is required.
              </span>
            )}
          </div>

          <div className="flex gap-3">
            <label className="flex flex-1 flex-col gap-1 text-body text-text">
              <span className="text-label text-text-secondary">Start date</span>
              <input
                type="date"
                value={startDate}
                onChange={e => setStartDate(e.target.value)}
                className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
              />
            </label>
            <label className="flex flex-1 flex-col gap-1 text-body text-text">
              <span className="text-label text-text-secondary">End date</span>
              <input
                type="date"
                value={endDate}
                onChange={e => setEndDate(e.target.value)}
                className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
              />
            </label>
          </div>

          {createEntryPersonMutation.isError && (
            <p className="text-body text-status-failure" role="alert">
              {conflict
                ? `${person.name} already holds the "${trimmedRole}" role.`
                : `Couldn't add this member (${createEntryPersonMutation.error.message}).`}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="h-9 px-4 rounded-lg bg-surface text-body font-medium text-text-secondary hover:text-text"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={createEntryPersonMutation.isPending}
              className="h-9 px-4 rounded-lg bg-accent-system text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {createEntryPersonMutation.isPending ? 'Adding…' : 'Add'}
            </button>
          </div>
        </form>
      </Modal>
    )
  }

  if (step === 'search') {
    return (
      <Modal title="Add member" onClose={onClose}>
        <div className="flex flex-col gap-4">
          <label className="relative">
            <Search
              size={16}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-text-secondary"
              aria-hidden="true"
            />
            <input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="Search people…"
              aria-label="Search people"
              autoFocus
              className="h-9 w-full rounded-lg border border-border bg-surface-raised pl-8 pr-3 text-body text-text focus:outline-none focus:border-border-hover"
            />
          </label>

          {searchQuery.isError && (
            <p className="text-body text-status-failure" role="alert">
              Couldn't search people ({searchQuery.error.message}).
            </p>
          )}

          {searchQuery.isPending && debouncedQuery.length >= MIN_QUERY_LENGTH && (
            <p className="text-body text-text-secondary">Searching…</p>
          )}

          {!searchQuery.isPending && !searchQuery.isError && debouncedQuery.length >= MIN_QUERY_LENGTH && results.length === 0 && (
            <EmptyState icon={SearchX} title="No matches" description="No people match your search." />
          )}

          {results.length > 0 && (
            <ul className="flex max-h-96 flex-col gap-1 overflow-y-auto">
              {results.map(candidate => (
                <li key={candidate.id}>
                  <button
                    type="button"
                    onClick={() => selectPerson({ id: candidate.id, name: candidate.name })}
                    className="flex w-full items-center gap-0.5 rounded-lg px-3 py-2 text-left hover:bg-surface"
                  >
                    <span className="text-body font-medium text-text">{candidate.name}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}

          <div className="flex justify-end">
            <button
              type="button"
              onClick={() => setStep('choose')}
              className="h-9 px-4 rounded-lg bg-surface text-body font-medium text-text-secondary hover:text-text"
            >
              Back
            </button>
          </div>
        </div>
      </Modal>
    )
  }

  return (
    <Modal title="Add member" onClose={onClose}>
      <div className="flex flex-col gap-2">
        <button
          type="button"
          onClick={() => setStep('search')}
          className="flex h-11 items-center rounded-lg border border-border px-4 text-body font-medium text-text hover:bg-surface-raised"
        >
          Search existing person
        </button>
        <button
          type="button"
          onClick={() => setStep('create')}
          className="flex h-11 items-center rounded-lg border border-border px-4 text-body font-medium text-text hover:bg-surface-raised"
        >
          Create new person
        </button>
      </div>
    </Modal>
  )
}
