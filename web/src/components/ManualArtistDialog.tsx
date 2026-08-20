import type { JsonObject } from '@bufbuild/protobuf'
import { useState } from 'react'
import { useMutation } from '@connectrpc/connect-query'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { createLibraryEntry } from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'
import { Modal } from './Modal'
import { TextInput } from './TextInput'

const ARTIST_TYPE_OPTIONS = ['Person', 'Group', 'Other'] as const
type ArtistType = (typeof ARTIST_TYPE_OPTIONS)[number]

interface FormState {
  name: string
  sortName: string
  artistType: ArtistType | ''
  country: string
  beginDate: string
  endDate: string
  overview: string
}

function initialState(): FormState {
  return { name: '', sortName: '', artistType: '', country: '', beginDate: '', endDate: '', overview: '' }
}

export interface ManualArtistDialogProps {
  onClose: () => void
  // onAdded fires once CreateLibraryEntry resolves — the caller decides
  // what "done" means (navigate, close, refresh), same contract
  // AddArtistDialog's onAdded already has.
  onAdded: (entry: LibraryEntry) => void
}

// ManualArtistDialog is #722's manual entry point: for an artist
// MusicBrainz doesn't have, added directly via
// LibraryEntryService.CreateLibraryEntry with no ExternalID and no
// MusicBrainz dedupe — nothing to dedupe against, same reasoning
// GroupDialog's own create mode and ManualEditionDialog already give.
// Kept as its own dialog rather than folded into EditArtistDialog: edit's
// field set (isni/official_url/wikipedia_url/aliases/lastfm_url) is
// provider-fetched data a manual add has none of, while create needs a
// required artist_type select and a type-conditional date pair edit mode
// doesn't. name is the only required field (domain.LibraryEntry.Validate());
// artist_type is additionally required here (UI-level: it picks which
// date pair applies) — monitored/monitor_mode are always true/ALL, same
// default-on-create every other manual-add dialog in this epic uses.
//
// artist_type=Person shows born_date/died_date; Group and Other both show
// founded_date/dissolved_date — the same isPerson rule ArtistDetail's own
// facts sidebar reads (artistType === 'Person' ? born/died : founded/
// dissolved, see web/src/pages/ArtistDetail.tsx).
export function ManualArtistDialog({ onClose, onAdded }: ManualArtistDialogProps) {
  const [form, setForm] = useState<FormState>(initialState)
  const [nameTouched, setNameTouched] = useState(false)
  const [artistTypeTouched, setArtistTypeTouched] = useState(false)
  const createMutation = useMutation(createLibraryEntry)

  const trimmedName = form.name.trim()
  const nameInvalid = trimmedName === ''
  const artistTypeInvalid = form.artistType === ''
  const canSubmit = !createMutation.isPending

  const isPerson = form.artistType === 'Person'
  const beginLabel = isPerson ? 'Born' : 'Founded'
  const endLabel = isPerson ? 'Died' : 'Dissolved'

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setNameTouched(true)
    setArtistTypeTouched(true)
    if (!canSubmit || nameInvalid || artistTypeInvalid) return

    const metadata: JsonObject = { artist_type: form.artistType }
    if (form.country !== '') metadata.country = form.country
    const beginKey = isPerson ? 'born_date' : 'founded_date'
    const endKey = isPerson ? 'died_date' : 'dissolved_date'
    if (form.beginDate !== '') metadata[beginKey] = form.beginDate
    if (form.endDate !== '') metadata[endKey] = form.endDate

    createMutation.mutate(
      {
        libraryEntry: {
          contentType: 'music',
          kind: 'artist',
          name: trimmedName,
          sortName: form.sortName,
          overview: form.overview,
          metadata,
          monitored: true,
          monitorMode: MonitorMode.ALL,
        },
      },
      { onSuccess: response => response.libraryEntry && onAdded(response.libraryEntry) },
    )
  }

  return (
    <Modal title="Add Artist Manually" onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <TextInput label="Name" value={form.name} onChange={v => update('name', String(v))} />
          {nameTouched && nameInvalid && (
            <span className="text-label text-status-failure" role="alert">
              Name is required.
            </span>
          )}
        </div>

        <TextInput label="Sort name" value={form.sortName} onChange={v => update('sortName', String(v))} />

        <div className="flex flex-col gap-1">
          <label className="flex flex-col gap-1 text-body text-text">
            <span className="text-label text-text-secondary">Artist type</span>
            <select
              value={form.artistType}
              onChange={e => update('artistType', e.target.value as ArtistType | '')}
              className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
            >
              <option value="">Select…</option>
              {ARTIST_TYPE_OPTIONS.map(opt => (
                <option key={opt} value={opt}>
                  {opt}
                </option>
              ))}
            </select>
          </label>
          {artistTypeTouched && artistTypeInvalid && (
            <span className="text-label text-status-failure" role="alert">
              Artist type is required.
            </span>
          )}
        </div>

        <TextInput label="Country" value={form.country} onChange={v => update('country', String(v))} />

        <div className="flex gap-3">
          <TextInput label={beginLabel} value={form.beginDate} onChange={v => update('beginDate', String(v))} />
          <TextInput label={endLabel} value={form.endDate} onChange={v => update('endDate', String(v))} />
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

        {createMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't add this artist ({createMutation.error.message}).
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
