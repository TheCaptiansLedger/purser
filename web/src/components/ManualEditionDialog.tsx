import { useMutation } from '@connectrpc/connect-query'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { useState } from 'react'
import { createMusicRelease } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import { ReleaseStatus, type Release } from '../gen/purser/music/v1/release_pb'
import { Modal } from './Modal'
import { TextInput } from './TextInput'

interface FormState {
  title: string
  country: string
  date: string
  label: string
  catalogNumber: string
  barcode: string
  format: string
  mediumCount: number | ''
  trackCount: number | ''
}

function initialState(): FormState {
  return { title: '', country: '', date: '', label: '', catalogNumber: '', barcode: '', format: '', mediumCount: '', trackCount: '' }
}

export interface ManualEditionDialogProps {
  // groupId/libraryEntryId — same "always the album already on screen"
  // scope AddEditionDialog has (#675).
  groupId: string
  libraryEntryId: string
  onClose: () => void
  onAdded: (release: Release) => void
}

// ManualEditionDialog is #675's manual entry point: for an edition
// MusicBrainz doesn't have, added directly via
// MusicReleaseService.CreateMusicRelease with no `mbid` field — the
// zero-value empty string, so MusicReleaseRepository.Create's plain
// `if rel.MBID == ""` branch runs (a plain inner.Create, never the
// get-or-create-on-MBID branch), same "no external identity to
// get-or-create against" reasoning ManualAlbumDialog's own doc comment
// gives. Title is the only required field (music.Release.Validate());
// status is always ReleaseStatusStub and is_default is always false —
// never track-derived, never auto-promoted (see #675's own issue body).
export function ManualEditionDialog({ groupId, libraryEntryId, onClose, onAdded }: ManualEditionDialogProps) {
  const [form, setForm] = useState<FormState>(initialState)
  const [titleTouched, setTitleTouched] = useState(false)
  const createMutation = useMutation(createMusicRelease)

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
        musicRelease: {
          groupId,
          libraryEntryId,
          title: trimmedTitle,
          country: form.country,
          date: form.date ? timestampFromDate(new Date(form.date)) : undefined,
          label: form.label,
          catalogNumber: form.catalogNumber,
          barcode: form.barcode,
          format: form.format,
          mediumCount: form.mediumCount === '' ? 0 : form.mediumCount,
          trackCount: form.trackCount === '' ? 0 : form.trackCount,
          status: ReleaseStatus.STUB,
          isDefault: false,
        },
      },
      { onSuccess: response => response.musicRelease && onAdded(response.musicRelease) },
    )
  }

  return (
    <Modal title="Add Edition Manually" onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <TextInput label="Title" value={form.title} onChange={v => update('title', String(v))} />
          {titleTouched && titleInvalid && (
            <span className="text-label text-status-failure" role="alert">
              Title is required.
            </span>
          )}
        </div>

        <div className="flex gap-3">
          <TextInput label="Country" value={form.country} onChange={v => update('country', String(v))} />
          <label className="flex flex-1 flex-col gap-1 text-body text-text">
            <span className="text-label text-text-secondary">Date</span>
            <input
              type="date"
              value={form.date}
              onChange={e => update('date', e.target.value)}
              className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
            />
          </label>
        </div>

        <div className="flex gap-3">
          <TextInput label="Label" value={form.label} onChange={v => update('label', String(v))} />
          <TextInput label="Catalog number" value={form.catalogNumber} onChange={v => update('catalogNumber', String(v))} />
        </div>

        <div className="flex gap-3">
          <TextInput label="Barcode" value={form.barcode} onChange={v => update('barcode', String(v))} />
          <TextInput label="Format" value={form.format} onChange={v => update('format', String(v))} />
        </div>

        <div className="flex gap-3">
          <TextInput
            label="Medium count"
            type="number"
            value={form.mediumCount}
            onChange={v => update('mediumCount', typeof v === 'number' && !Number.isNaN(v) ? v : '')}
          />
          <TextInput
            label="Track count"
            type="number"
            value={form.trackCount}
            onChange={v => update('trackCount', typeof v === 'number' && !Number.isNaN(v) ? v : '')}
          />
        </div>

        {createMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't add this edition ({createMutation.error.message}).
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
