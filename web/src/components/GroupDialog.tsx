import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import type { Group } from '../gen/purser/domain/v1/group_pb'
import { createGroup, updateGroup } from '../gen/purser/domain/v1/group-GroupService_connectquery'
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

// FIELD_PATHS pairs each editable, non-monitor field with the proto
// field-mask path applyGroupFieldMask (internal/api/connect/group_convert.go)
// switches on. monitored/monitorMode are deliberately excluded here — #681
// excludes them from the edit surface since the Album/Discography grids
// already own that control; they still appear as a create-time default,
// handled as a one-off in handleSubmit rather than through this diffing
// loop.
const FIELD_PATHS: { key: Exclude<keyof FormState, 'monitored'>; path: string }[] = [
  { key: 'title', path: 'title' },
  { key: 'sortName', path: 'sort_name' },
  { key: 'number', path: 'number' },
  { key: 'year', path: 'year' },
  { key: 'overview', path: 'overview' },
]

function initialState(group?: Group): FormState {
  if (!group) {
    return { title: '', sortName: '', number: '', year: '', overview: '', monitored: true }
  }
  return {
    title: group.title,
    sortName: group.sortName,
    number: group.number,
    year: group.year > 0 ? group.year : '',
    overview: group.overview,
    monitored: group.monitored,
  }
}

export interface GroupDialogProps {
  mode: 'create' | 'edit'
  // artistId — the owning LibraryEntry.ID, required in create mode
  // (matches ManualAlbumDialog's prior artistId prop); ignored in edit
  // mode, where the group already has its own libraryEntryId.
  artistId?: string
  // group — required in edit mode (the record being edited); ignored in
  // create mode.
  group?: Group
  onClose: () => void
  onSaved: (group: Group) => void
}

// GroupDialog is #669's manual-entry "Add Album Manually" dialog
// (originally ManualAlbumDialog, create-only) generalized by #681 into a
// create/edit dialog on the same PersonDialog (#663) pattern: one form,
// `mode` picks CreateGroup vs a minimal-field-mask UpdateGroup. Fields are
// title/sort_name/number/year/overview — #681's own Album field list.
// Monitored is create-only (a sane default for a brand-new album); edit
// mode never renders or touches it, since ArtistDetail's Discography grid
// and AlbumDetail's own toggle already own that control (#681's explicit
// exclusion, mirroring Artist/Person's own monitor toggles).
export function GroupDialog({ mode, artistId, group, onClose, onSaved }: GroupDialogProps) {
  const [initial] = useState(() => initialState(mode === 'edit' ? group : undefined))
  const [form, setForm] = useState<FormState>(initial)
  const [titleTouched, setTitleTouched] = useState(false)
  const createMutation = useMutation(createGroup)
  const updateMutation = useMutation(updateGroup)
  const mutation = mode === 'create' ? createMutation : updateMutation

  const trimmedTitle = form.title.trim()
  const titleInvalid = trimmedTitle === ''

  const changedPaths =
    mode === 'edit' ? FIELD_PATHS.filter(({ key }) => form[key] !== initial[key]).map(({ path }) => path) : []
  const canSubmit = mode === 'create' ? !mutation.isPending : !mutation.isPending && changedPaths.length > 0

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setTitleTouched(true)
    if (!canSubmit || titleInvalid) return

    const wire = {
      title: trimmedTitle,
      sortName: form.sortName,
      number: form.number,
      year: form.year === '' ? 0 : form.year,
      overview: form.overview,
    }

    if (mode === 'create') {
      createMutation.mutate(
        {
          group: {
            libraryEntryId: artistId ?? '',
            ...wire,
            monitored: form.monitored,
            monitorMode: form.monitored ? MonitorMode.ALL : MonitorMode.NONE,
          },
        },
        { onSuccess: response => response.group && onSaved(response.group) },
      )
      return
    }

    updateMutation.mutate(
      { group: { id: group?.id ?? '', ...wire }, updateMask: { paths: changedPaths } },
      { onSuccess: response => response.group && onSaved(response.group) },
    )
  }

  return (
    <Modal title={mode === 'create' ? 'Add Album Manually' : 'Edit Album'} onClose={onClose}>
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

        {mode === 'create' && (
          <Toggle label="Monitored" checked={form.monitored} onChange={v => update('monitored', v)} />
        )}

        {mutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't save this album ({mutation.error.message}).
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
            {mutation.isPending ? 'Saving…' : mode === 'create' ? 'Add' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
