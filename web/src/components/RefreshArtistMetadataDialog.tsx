import { useState } from 'react'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { useRefreshArtistMetadata } from '../hooks/useRefreshArtistMetadata'
import { Modal } from './Modal'

export interface RefreshArtistMetadataDialogProps {
  entry: LibraryEntry
  // mbid — the caller is responsible for not rendering this dialog until
  // it's known, same precedent AddAlbumDialog's artistMbid prop sets.
  mbid: string
  onClose: () => void
  // onUpdated fires once the apply mutation resolves — the caller decides
  // what "done" means (ArtistDetail refetches its LibraryEntry).
  onUpdated: (entry: LibraryEntry) => void
}

function formatValue(value: string | string[] | undefined): string {
  if (value === undefined) return '—'
  return Array.isArray(value) ? value.join(', ') : value
}

// RefreshArtistMetadataDialog is #671's confirm surface:
// useRefreshArtistMetadata's diff, presented one row per changed field
// (current struck through, proposed highlighted), Confirm submits exactly
// that diff's field-mask via the same hook's apply(). A no-diff result
// renders "Already up to date" instead of an empty confirm with nothing
// to approve, per this issue's acceptance criterion.
export function RefreshArtistMetadataDialog({ entry, mbid, onClose, onUpdated }: RefreshArtistMetadataDialogProps) {
  const { isPending, isError, error, diff, apply, isApplying } = useRefreshArtistMetadata(entry, mbid)
  const [applyError, setApplyError] = useState<string | null>(null)

  async function handleConfirm() {
    setApplyError(null)
    try {
      const updated = await apply()
      onUpdated(updated ?? entry)
    } catch (err) {
      setApplyError(err instanceof Error ? err.message : 'Could not update this artist.')
    }
  }

  return (
    <Modal title="Refresh from MusicBrainz" onClose={onClose}>
      <div className="flex flex-col gap-4">
        {isPending && <p className="text-body text-text-secondary">Checking MusicBrainz…</p>}

        {isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't reach MusicBrainz ({error?.message}).
          </p>
        )}

        {!isPending && !isError && diff.length === 0 && (
          <p className="text-body text-text-secondary">Already up to date.</p>
        )}

        {!isPending && !isError && diff.length > 0 && (
          <ul className="flex flex-col gap-3">
            {diff.map(field => (
              <li key={`${field.path}.${field.metadataKey ?? ''}`} className="flex flex-col gap-0.5">
                <span className="text-label font-medium text-text-secondary">{field.label}</span>
                <span className="text-body">
                  <span className="text-text-secondary line-through">{formatValue(field.current)}</span>
                  <span className="mx-2 text-text-secondary">→</span>
                  <span className="font-medium text-accent-system">{formatValue(field.proposed)}</span>
                </span>
              </li>
            ))}
          </ul>
        )}

        {applyError && (
          <p className="text-body text-status-failure" role="alert">
            {applyError}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="flex h-9 items-center rounded-lg px-4 text-body font-medium text-text-secondary hover:bg-surface-raised hover:text-text"
          >
            {diff.length === 0 ? 'Close' : 'Cancel'}
          </button>
          {diff.length > 0 && (
            <button
              type="button"
              onClick={handleConfirm}
              disabled={isApplying}
              className="flex h-9 items-center rounded-lg bg-accent-system px-4 text-body font-medium text-bg hover:opacity-90 disabled:opacity-50"
            >
              {isApplying ? 'Updating…' : 'Confirm'}
            </button>
          )}
        </div>
      </div>
    </Modal>
  )
}
