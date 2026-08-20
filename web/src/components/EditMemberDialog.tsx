import { ConnectError, Code } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import { timestampDate, timestampFromDate } from '@bufbuild/protobuf/wkt'
import type { Timestamp } from '@bufbuild/protobuf/wkt'
import { useState } from 'react'
import {
  createEntryPerson,
  deleteEntryPerson,
  updateEntryPerson,
} from '../gen/purser/domain/v1/entry_person-EntryPersonService_connectquery'
import { Modal } from './Modal'
import { TextInput } from './TextInput'

function toDateInput(timestamp?: Timestamp): string {
  return timestamp ? timestampDate(timestamp).toISOString().slice(0, 10) : ''
}

export interface EditMemberDialogProps {
  libraryEntryId: string
  personId: string
  personName: string
  role: string
  startDate?: Timestamp
  endDate?: Timestamp
  onClose: () => void
  onSaved: () => void
}

// EditMemberDialog is #724's Edit action on a Members-tab row — role plus
// era (start_date/end_date), pre-filled from the EntryPerson row it opened
// against.
//
// role is part of EntryPerson's identity key (library_entry_id, person_id,
// role) — see entry_person_convert.go's applyEntryPersonFieldMask, which
// deliberately has no "role" case, so UpdateEntryPerson can never rename a
// row in place. Two paths, chosen by whether Role was touched:
//
//  - Role unchanged: a plain UpdateEntryPerson with a field mask of
//    whichever date fields actually changed (same minimal-mask convention
//    #663/#671 already use).
//  - Role changed: CreateEntryPerson under the new role first, then
//    DeleteEntryPerson the old row — create-before-delete so a collision
//    with an already-existing (person, role) pair (surfaced inline, same
//    as Add Member's own AlreadyExists case) leaves the original row
//    untouched rather than losing it. If the old row's delete itself then
//    fails, the member is left holding both roles rather than silently
//    disappearing — surfaced as an error, but not treated as fatal, since
//    the requested new role was in fact saved.
export function EditMemberDialog({
  libraryEntryId,
  personId,
  personName,
  role,
  startDate,
  endDate,
  onClose,
  onSaved,
}: EditMemberDialogProps) {
  const [roleInput, setRoleInput] = useState(role)
  const [roleTouched, setRoleTouched] = useState(false)
  const [startDateInput, setStartDateInput] = useState(toDateInput(startDate))
  const [endDateInput, setEndDateInput] = useState(toDateInput(endDate))
  const [cleanupError, setCleanupError] = useState<string | null>(null)

  const updateMutation = useMutation(updateEntryPerson)
  const createMutation = useMutation(createEntryPerson)
  const deleteMutation = useMutation(deleteEntryPerson)

  const trimmedRole = roleInput.trim()
  const roleInvalid = trimmedRole === ''
  const roleChanged = trimmedRole !== role
  const startChanged = startDateInput !== toDateInput(startDate)
  const endChanged = endDateInput !== toDateInput(endDate)
  const hasChanges = roleChanged || startChanged || endChanged
  const isPending = updateMutation.isPending || createMutation.isPending || deleteMutation.isPending
  const canSubmit = !isPending && hasChanges

  const newStartDate = startDateInput ? timestampFromDate(new Date(startDateInput)) : undefined
  const newEndDate = endDateInput ? timestampFromDate(new Date(endDateInput)) : undefined

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setRoleTouched(true)
    if (roleInvalid || !canSubmit) return
    setCleanupError(null)

    if (!roleChanged) {
      const paths = [
        ...(startChanged ? ['start_date'] : []),
        ...(endChanged ? ['end_date'] : []),
      ]
      updateMutation.mutate(
        {
          entryPerson: { libraryEntryId, personId, role, startDate: newStartDate, endDate: newEndDate },
          updateMask: { paths },
        },
        { onSuccess: onSaved },
      )
      return
    }

    try {
      await createMutation.mutateAsync({
        entryPerson: { libraryEntryId, personId, role: trimmedRole, startDate: newStartDate, endDate: newEndDate },
      })
    } catch {
      // Surfaced below via createMutation.isError — the old row is
      // untouched, nothing to clean up.
      return
    }

    try {
      await deleteMutation.mutateAsync({ libraryEntryId, personId, role })
    } catch (err) {
      setCleanupError(
        `Added the "${trimmedRole}" role, but couldn't remove the old "${role}" role (${ConnectError.from(err).message}).`,
      )
    }
    onSaved()
  }

  const conflict = createMutation.isError && ConnectError.from(createMutation.error).code === Code.AlreadyExists

  return (
    <Modal title={`Edit ${personName}'s role`} onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <TextInput label="Role" value={roleInput} onChange={v => setRoleInput(String(v))} />
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
              value={startDateInput}
              onChange={e => setStartDateInput(e.target.value)}
              className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
            />
          </label>
          <label className="flex flex-1 flex-col gap-1 text-body text-text">
            <span className="text-label text-text-secondary">End date</span>
            <input
              type="date"
              value={endDateInput}
              onChange={e => setEndDateInput(e.target.value)}
              className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
            />
          </label>
        </div>

        {createMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            {conflict
              ? `${personName} already holds the "${trimmedRole}" role.`
              : `Couldn't save this role (${createMutation.error.message}).`}
          </p>
        )}

        {updateMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't save this role ({updateMutation.error.message}).
          </p>
        )}

        {cleanupError && (
          <p className="text-body text-status-failure" role="alert">
            {cleanupError}
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
            disabled={!canSubmit}
            className="h-9 px-4 rounded-lg bg-accent-system text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isPending ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
