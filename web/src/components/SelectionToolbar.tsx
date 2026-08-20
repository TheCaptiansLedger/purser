export interface SelectionToolbarProps {
  count: number
  entityLabelPlural: string
  onDelete: () => void
  onCancel: () => void
  // Monitor/Unmonitor (#680): both optional and only rendered together —
  // a call site either wants the bulk monitor-toggle actions or doesn't,
  // there's no single-direction case. People's bulk-delete usage (#679)
  // omits both and is unaffected.
  onMonitor?: () => void
  onUnmonitor?: () => void
  isUpdatingMonitored?: boolean
}

// SelectionToolbar — the bulk-select action bar (#679) shared between the
// Music Library grid and the Discography tab: a live selection count plus
// Delete/Cancel. Delete is disabled at count=0 rather than hidden, so the
// bar's layout doesn't jump as the selection changes.
//
// Monitor/Unmonitor (#680): explicit set-true/set-false actions rather
// than a single "toggle" — a mixed selection (some monitored, some not)
// has no sensible single toggle direction. Unlike Delete, these fire
// directly with no confirm dialog: the underlying client-side loop over
// UpdateLibraryEntry/UpdateGroup (see MusicLibrary/ArtistDetail) is safely
// retryable and non-destructive, per #680's own ADR 0016 justification.
export function SelectionToolbar({
  count,
  entityLabelPlural,
  onDelete,
  onCancel,
  onMonitor,
  onUnmonitor,
  isUpdatingMonitored = false,
}: SelectionToolbarProps) {
  const showMonitorActions = onMonitor && onUnmonitor

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
        {showMonitorActions && (
          <>
            <button
              type="button"
              onClick={onUnmonitor}
              disabled={count === 0 || isUpdatingMonitored}
              className="h-9 px-4 rounded-lg border border-border text-body font-medium text-text hover:bg-surface disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Unmonitor
            </button>
            <button
              type="button"
              onClick={onMonitor}
              disabled={count === 0 || isUpdatingMonitored}
              className="h-9 px-4 rounded-lg border border-border text-body font-medium text-text hover:bg-surface disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Monitor
            </button>
          </>
        )}
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
