import { useMutation } from '@connectrpc/connect-query'
import { deleteEntryPerson } from '../gen/purser/domain/v1/entry_person-EntryPersonService_connectquery'
import { Modal } from './Modal'

export interface RemoveMemberDialogProps {
  libraryEntryId: string
  personId: string
  personName: string
  role: string
  onClose: () => void
  onRemoved: () => void
}

// RemoveMemberDialog is #724's Remove action on a Members-tab row — a
// plain confirm, no GetDeletionImpact query. EntryPerson is a pure join
// row with no referrers of its own (ADR 0015's own scope note), so unlike
// PersonDeleteDialog/TrackDeleteDialog there's nothing to unlink or
// preview here: DeleteEntryPerson is a hard delete of just this one row.
export function RemoveMemberDialog({ libraryEntryId, personId, personName, role, onClose, onRemoved }: RemoveMemberDialogProps) {
  const deleteMutation = useMutation(deleteEntryPerson)

  function handleRemove() {
    deleteMutation.mutate({ libraryEntryId, personId, role }, { onSuccess: onRemoved })
  }

  return (
    <Modal title={`Remove ${personName}?`} onClose={onClose}>
      <div className="flex flex-col gap-4">
        <p className="text-body text-text-secondary">
          Remove {personName} from the &quot;{role}&quot; role? This cannot be undone.
        </p>

        {deleteMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't remove this member ({deleteMutation.error.message}).
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
            type="button"
            onClick={handleRemove}
            disabled={deleteMutation.isPending}
            className="h-9 px-4 rounded-lg bg-status-failure text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {deleteMutation.isPending ? 'Removing…' : 'Remove'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
