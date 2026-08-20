export interface BulkActionError {
  id: string
  label: string
  message: string
}

export interface BulkActionErrorsProps {
  errors: BulkActionError[]
}

// BulkActionErrors — per-row failure surface for the bulk monitor-toggle
// (#680). The underlying action is a client-side Promise.allSettled loop
// over UpdateLibraryEntry/UpdateGroup (see MusicLibrary/ArtistDetail), not
// a transactional bulk RPC (see ADR 0016 exception this issue documents),
// so a partial outcome is a real, expected case — this renders every
// failed row by name, never a single collapsed all-or-nothing message
// that would misrepresent it.
export function BulkActionErrors({ errors }: BulkActionErrorsProps) {
  if (errors.length === 0) return null

  return (
    <div
      role="alert"
      className="mt-2 flex flex-col gap-1 rounded-lg border border-status-failure/40 bg-surface-raised px-4 py-2"
    >
      <span className="text-body font-medium text-status-failure">
        Couldn&apos;t update {errors.length} {errors.length === 1 ? 'item' : 'items'}
      </span>
      <ul className="flex flex-col gap-0.5">
        {errors.map(error => (
          <li key={error.id} className="text-label text-status-failure">
            {error.label}: {error.message}
          </li>
        ))}
      </ul>
    </div>
  )
}
