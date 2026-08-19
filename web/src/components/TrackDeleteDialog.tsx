import { useMutation, useQuery } from '@connectrpc/connect-query'
import { deleteItem, getItemDeletionImpact } from '../gen/purser/domain/v1/item-ItemService_connectquery'
import { Modal } from './Modal'

export interface TrackDeleteDialogProps {
  trackId: string
  trackTitle: string
  onClose: () => void
  onDeleted: () => void
}

// TrackDeleteDialog is #676's track Delete confirm — the generic
// ItemService.DeleteItem RPC, gated by GetItemDeletionImpact per ADR 0015,
// same pattern every entity with potential referrers uses (a track with a
// real MediaFile attached shows that row here). Item never blocks a
// delete (every referrer is a pure attachment row — see
// internal/service/item_deletion.go's own doc comment), so every impact
// row here is informational, not a hard stop; Delete always unlinks
// referrers first, never destroys them.
export function TrackDeleteDialog({ trackId, trackTitle, onClose, onDeleted }: TrackDeleteDialogProps) {
  const impactQuery = useQuery(getItemDeletionImpact, { id: trackId })
  const deleteMutation = useMutation(deleteItem)

  const impacts = (impactQuery.data?.impacts ?? []).filter(row => row.count > 0)

  function handleDelete() {
    deleteMutation.mutate({ id: trackId, cascade: false }, { onSuccess: onDeleted })
  }

  return (
    <Modal title={`Delete "${trackTitle}"?`} onClose={onClose}>
      <div className="flex flex-col gap-4">
        {impactQuery.isPending && <p className="text-body text-text-secondary">Checking what references this track…</p>}

        {impactQuery.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't check this track's references ({impactQuery.error.message}).
          </p>
        )}

        {!impactQuery.isPending && !impactQuery.isError && (
          <>
            {impacts.length === 0 ? (
              <p className="text-body text-text-secondary">Nothing else references this track.</p>
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
            Couldn't delete this track ({deleteMutation.error.message}).
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
