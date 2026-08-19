import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { createMusicReleaseTrack } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import type { Item } from '../gen/purser/domain/v1/item_pb'
import { Modal } from './Modal'
import { TextInput } from './TextInput'

interface FormState {
  title: string
  number: string
  mediumNumber: number
  runtimeSeconds: number | ''
}

function initialState(): FormState {
  return { title: '', number: '', mediumNumber: 1, runtimeSeconds: '' }
}

export interface AddTrackDialogProps {
  releaseId: string
  onClose: () => void
  onAdded: (track: Item) => void
}

// AddTrackDialog is #676's "Add track manually" entry point:
// MusicReleaseService.CreateMusicReleaseTrack with no mbid — a hand-entered
// track carries no recording identity (#721's own scope note). The created
// track always lands Status=missing server-side (#721's CreateTrack, never
// wanted) — this dialog has no status field to set. medium_number defaults
// to 1 (single-disc is the common case), title is the only required field.
export function AddTrackDialog({ releaseId, onClose, onAdded }: AddTrackDialogProps) {
  const [form, setForm] = useState<FormState>(initialState)
  const [titleTouched, setTitleTouched] = useState(false)
  const createMutation = useMutation(createMusicReleaseTrack)

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
        releaseId,
        title: trimmedTitle,
        number: form.number.trim(),
        mediumNumber: form.mediumNumber,
        runtimeSeconds: form.runtimeSeconds === '' ? 0 : form.runtimeSeconds,
      },
      { onSuccess: response => response.track && onAdded(response.track) },
    )
  }

  return (
    <Modal title="Add Track Manually" onClose={onClose}>
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
          <TextInput label="Number" value={form.number} onChange={v => update('number', String(v))} hint='e.g. "3" or "A2"' />
          <TextInput
            label="Medium number"
            type="number"
            value={form.mediumNumber}
            onChange={v => update('mediumNumber', typeof v === 'number' && !Number.isNaN(v) ? v : 1)}
          />
        </div>

        <TextInput
          label="Runtime"
          type="number"
          hint="seconds"
          value={form.runtimeSeconds}
          onChange={v => update('runtimeSeconds', typeof v === 'number' && !Number.isNaN(v) ? v : '')}
        />

        {createMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't add this track ({createMutation.error.message}).
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
