export interface SelectionToolbarProps {
  count: number
  entityLabelPlural: string
  onDelete: () => void
  onCancel: () => void
}

// SelectionToolbar — the bulk-select action bar (#679) shared between the
// Music Library grid and the Discography tab: a live selection count plus
// Delete/Cancel. Delete is disabled at count=0 rather than hidden, so the
// bar's layout doesn't jump as the selection changes.
export function SelectionToolbar({ count, entityLabelPlural, onDelete, onCancel }: SelectionToolbarProps) {
  return (
    <div className="mt-4 flex items-center justify-between rounded-lg border border-border bg-surface-raised px-4 py-2">
      <span className="text-body text-text">
        {count} {entityLabelPlural} selected
      </span>
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={onCancel}
          className="h-9 px-4 rounded-lg text-body font-medium text-text-secondary hover:text-text"
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={onDelete}
          disabled={count === 0}
          className="h-9 px-4 rounded-lg bg-status-failure text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Delete
        </button>
      </div>
    </div>
  )
}
