import { useMutation, useQuery } from '@connectrpc/connect-query'
import { deletePerson, getPersonDeletionImpact } from '../gen/purser/domain/v1/person-PersonService_connectquery'
import { Modal } from './Modal'

export interface PersonDeleteDialogProps {
  personId: string
  personName: string
  onClose: () => void
  onDeleted: () => void
}

// PersonDeleteDialog is Person Detail's single-delete confirm — the
// generic PersonService.DeletePerson RPC, gated by GetPersonDeletionImpact
// per ADR 0015, same pattern TrackDeleteDialog already set. Person never
// blocks a delete (every referrer is a pure attachment row — see
// internal/service/person_deletion.go's own doc comment), so every impact
// row here is informational, not a hard stop, and cascade is always sent
// false — there's nothing for it to affect.
export function PersonDeleteDialog({ personId, personName, onClose, onDeleted }: PersonDeleteDialogProps) {
  const impactQuery = useQuery(getPersonDeletionImpact, { id: personId })
  const deleteMutation = useMutation(deletePerson)

  const impacts = (impactQuery.data?.impacts ?? []).filter(row => row.count > 0)

  function handleDelete() {
    deleteMutation.mutate({ id: personId, cascade: false }, { onSuccess: onDeleted })
  }

  return (
    <Modal title={`Delete "${personName}"?`} onClose={onClose}>
      <div className="flex flex-col gap-4">
        {impactQuery.isPending && <p className="text-body text-text-secondary">Checking what references this person…</p>}

        {impactQuery.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't check this person's references ({impactQuery.error.message}).
          </p>
        )}

        {!impactQuery.isPending && !impactQuery.isError && (
          <>
            {impacts.length === 0 ? (
              <p className="text-body text-text-secondary">Nothing else references this person.</p>
            ) : (
              <ul className="flex flex-col gap-1">
                {impacts.map(row => (
                  <li key={row.kind} className="flex items-center justify-between text-body text-text">
                    <span>{row.label}</span>
                    <span className="text-text-secondary">{row.count}</span>
                  </li>
                ))}
              </ul>
            )}
            <p className="text-body text-text-secondary">This cannot be undone.</p>
          </>
        )}

        {deleteMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't delete this person ({deleteMutation.error.message}).
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
            onClick={handleDelete}
            disabled={impactQuery.isPending || deleteMutation.isPending}
            className="h-9 px-4 rounded-lg bg-status-failure text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
