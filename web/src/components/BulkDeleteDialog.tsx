import { useState } from 'react'
import type { AggregatedImpactRow } from '../lib/deletionImpact'
import { Modal } from './Modal'

export interface BulkDeleteDialogProps {
  entityLabelPlural: string
  count: number
  impact: { isPending: boolean; rows: AggregatedImpactRow[] }
  // Only ever shown when a blocking row exists — Group's own bulk delete
  // never triggers it (Group never blocks, see group_deletion.go), so
  // that call site accepts this prop's default rather than supplying its
  // own text for a checkbox it never renders.
  cascadeLabel?: string
  onDelete: (cascade: boolean) => void
  isDeleting: boolean
  deleteError?: string
  onClose: () => void
}

// BulkDeleteDialog — the bulk-select confirm dialog (#679), shared
// between the Music Library grid (LibraryEntry) and the Discography tab
// (Group). Purely presentational: it takes already-aggregated impact data
// and plain callbacks rather than an RPC method descriptor, so it carries
// no connect-query/entity-specific knowledge — each page supplies its own
// useLibraryEntryDeletionImpacts/useGroupDeletionImpacts result and its
// own bulkDeleteLibraryEntries/bulkDeleteGroups mutation.
//
// Same TrackDeleteDialog precedent (impact list, "cannot be undone",
// Cancel/Delete), extended for the bulk/blocking case per ADR 0015: a
// non-blocking row is always informational (Delete always unlinks it);
// a blocking row (e.g. LibraryEntry's Groups/Items) means at least one
// selected row would fail its own single-row Delete precondition, so
// Delete stays disabled until the caller explicitly opts into cascade via
// the checkbox this dialog shows only when a blocking row exists — never
// checked by default, per ADR 0015's "Cascade is opt-in, never the
// default."
//
// All-or-nothing per ADR 0016: there is no partial-success state to
// render here — a failed bulk delete leaves every selected row untouched,
// so the only outcomes are "still open with an error" or "onDelete's
// caller reports success and this closes."
export function BulkDeleteDialog({
  entityLabelPlural,
  count,
  impact,
  cascadeLabel = 'Also delete everything that blocks this',
  onDelete,
  isDeleting,
  deleteError,
  onClose,
}: BulkDeleteDialogProps) {
  const [cascade, setCascade] = useState(false)

  const blockingRows = impact.rows.filter(row => row.blocking)
  const nonBlockingRows = impact.rows.filter(row => !row.blocking)
  const hasBlocking = blockingRows.length > 0

  const deleteDisabled = impact.isPending || isDeleting || (hasBlocking && !cascade)

  return (
    <Modal title={`Delete ${count} ${entityLabelPlural}?`} onClose={onClose}>
      <div className="flex flex-col gap-4">
        {impact.isPending && (
          <p className="text-body text-text-secondary">Checking what references these {entityLabelPlural}…</p>
        )}

        {!impact.isPending && (
          <>
            {impact.rows.length === 0 ? (
              <p className="text-body text-text-secondary">Nothing else references these {entityLabelPlural}.</p>
            ) : (
              <ul className="flex flex-col gap-1">
                {blockingRows.map(row => (
                  <li key={row.kind} className="flex items-center justify-between text-body text-status-failure">
                    <span>{row.label}</span>
                    <span>{row.count}</span>
                  </li>
                ))}
                {nonBlockingRows.map(row => (
                  <li key={row.kind} className="flex items-center justify-between text-body text-text">
                    <span>{row.label}</span>
                    <span className="text-text-secondary">{row.count}</span>
                  </li>
                ))}
              </ul>
            )}

            {hasBlocking && (
              <label className="flex items-start gap-2 text-body text-text">
                <input
                  type="checkbox"
                  checked={cascade}
                  onChange={e => setCascade(e.target.checked)}
                  className="mt-1"
                />
                {cascadeLabel}
              </label>
            )}

            <p className="text-body text-text-secondary">This cannot be undone.</p>
          </>
        )}

        {deleteError && (
          <p className="text-body text-status-failure" role="alert">
            {deleteError}
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
            onClick={() => onDelete(cascade)}
            disabled={deleteDisabled}
            className="h-9 px-4 rounded-lg bg-status-failure text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isDeleting ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
