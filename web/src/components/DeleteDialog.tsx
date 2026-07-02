import { useState, useEffect } from 'react'
import { getDeletionPreview, delEntityConfirmed } from '../api/client'
import type { DeletionImpact } from '../types'

export function deleteDialogHeading(mode: 'destroy' | 'unlink', entityName: string): string {
  return mode === 'destroy' ? `Delete ${entityName}` : `Remove ${entityName}`
}

export function deleteButtonLabel(mode: 'destroy' | 'unlink', deleting: boolean): string {
  if (deleting) return 'Deleting…'
  return mode === 'destroy' ? 'Delete' : 'Remove'
}

interface Props {
  open: boolean
  entityName: string
  resource: string
  entityId: string
  onClose: () => void
  onDeleted: () => void
}

export default function DeleteDialog({ open, entityName, resource, entityId, onClose, onDeleted }: Props) {
  const [impact, setImpact] = useState<DeletionImpact | null>(null)
  const [loading, setLoading] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {
      setImpact(null)
      setError(null)
      return
    }
    setLoading(true)
    getDeletionPreview(resource, entityId)
      .then(setImpact)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [open, resource, entityId])

  async function handleConfirm() {
    setDeleting(true)
    setError(null)
    try {
      await delEntityConfirmed(resource, entityId)
      onDeleted()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Delete failed')
      setDeleting(false)
    }
  }

  if (!open) return null

  const mode = impact?.mode ?? 'destroy'
  const heading = deleteDialogHeading(mode, entityName)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-zinc-900 border border-zinc-700 rounded-lg shadow-xl w-full max-w-md p-6 space-y-4">
        <h2 className="text-lg font-semibold text-white">{heading}</h2>

        {loading && (
          <p className="text-zinc-400 text-sm">Loading impact…</p>
        )}

        {!loading && impact && (
          <>
            <p className="text-zinc-300 text-sm">{impact.summary}</p>

            {impact.impacts.length > 0 && (
              <ul className="space-y-1">
                {impact.impacts.map((row) => (
                  <li key={row.kind} className="flex justify-between text-sm">
                    <span className="text-zinc-400">{row.label}</span>
                    <span className="text-white font-medium">{row.count}</span>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}

        {error && (
          <p className="text-red-400 text-sm">{error}</p>
        )}

        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            disabled={deleting}
            className="px-4 py-2 text-sm rounded bg-zinc-700 hover:bg-zinc-600 text-white disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={loading || deleting || !!error}
            className="px-4 py-2 text-sm rounded bg-red-600 hover:bg-red-700 text-white disabled:opacity-50"
          >
            {deleteButtonLabel(mode, deleting)}
          </button>
        </div>
      </div>
    </div>
  )
}
