import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import type { Group } from '../gen/purser/domain/v1/group_pb'
import { createGroup } from '../gen/purser/domain/v1/group-GroupService_connectquery'
import { Modal } from './Modal'
import { TextInput } from './TextInput'
import { Toggle } from './Toggle'

interface FormState {
  title: string
  sortName: string
  number: string
  year: number | ''
  overview: string
  monitored: boolean
}

function initialState(): FormState {
  return { title: '', sortName: '', number: '', year: '', overview: '', monitored: true }
}

export interface ManualAlbumDialogProps {
  // artistId is the current artist's LibraryEntry.ID — same "always the
  // artist already on screen" scope AddAlbumDialog has (#669).
  artistId: string
  onClose: () => void
  onAdded: (group: Group) => void
}

// ManualAlbumDialog is #669's manual entry point: for an album MusicBrainz
// doesn't have, added directly via GroupService.CreateGroup with no
// `ExternalID` — there is no external identity to get-or-create against
// (ADR 0026's 3-step dance exists specifically to dedupe imports of the
// *same* external identity; a manual entry has none). Fields are exactly
// #681's own planned Album-editor field list (title, sort_name, number,
// year, overview) so this form and that future editor never drift on
// what's editable — manual-entry-only, no provider search, mirrors
// PersonDialog's (#663) own split from its provider-search sibling.
export function ManualAlbumDialog({ artistId, onClose, onAdded }: ManualAlbumDialogProps) {
  const [form, setForm] = useState<FormState>(initialState)
  const [titleTouched, setTitleTouched] = useState(false)
  const createMutation = useMutation(createGroup)

  const trimmedTitle = form.title.trim()
  const titleInvalid = trimmedTitle === ''
  const canSubmit = !createMutation.isPending

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setTitleTouched(true)
    if (!canSubmit || titleInvalid) return

    createMutation.mutate(
      {
        group: {
          libraryEntryId: artistId,
          title: trimmedTitle,
          sortName: form.sortName,
          number: form.number,
          year: form.year === '' ? 0 : form.year,
          overview: form.overview,
          monitored: form.monitored,
          monitorMode: form.monitored ? MonitorMode.ALL : MonitorMode.NONE,
        },
      },
      { onSuccess: response => response.group && onAdded(response.group) },
    )
  }

  return (
    <Modal title="Add Album Manually" onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <TextInput label="Title" value={form.title} onChange={v => update('title', String(v))} />
          {titleTouched && titleInvalid && (
            <span className="text-label text-status-failure" role="alert">
              Title is required.
            </span>
          )}
        </div>

        <TextInput label="Sort title" value={form.sortName} onChange={v => update('sortName', String(v))} />

        <div className="flex gap-3">
          <TextInput label="Number" value={form.number} onChange={v => update('number', String(v))} />
          <TextInput
            label="Year"
            type="number"
            value={form.year}
            onChange={v => update('year', typeof v === 'number' && !Number.isNaN(v) ? v : '')}
          />
        </div>

        <label className="flex flex-col gap-1 text-body text-text">
          <span className="text-label text-text-secondary">Overview</span>
          <textarea
            value={form.overview}
            onChange={e => update('overview', e.target.value)}
            rows={3}
            className="px-3 py-2 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          />
        </label>

        <Toggle label="Monitored" checked={form.monitored} onChange={v => update('monitored', v)} />

        {createMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't add this album ({createMutation.error.message}).
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
            {createMutation.isPending ? 'Adding…' : 'Add'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
