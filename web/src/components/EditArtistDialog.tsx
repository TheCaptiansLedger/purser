import type { JsonObject } from '@bufbuild/protobuf'
import { useState } from 'react'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { aliasesField, stringField } from '../lib/metadataFields'
import { useUpdateLibraryEntryMutation } from '../hooks/useLibraryEntry'
import { ListInput } from './ListInput'
import { Modal } from './Modal'
import { TextInput } from './TextInput'

// METADATA_KEYS — the identity-fact Metadata sub-keys #681 names as
// editable, distinct from the bio/backdrop data ArtistDetail fetches live
// from providers per ADR 0027 (those are never persisted, so never
// editable here). Same key names useRefreshArtistMetadata.ts already
// writes, minus the born/died-vs-founded/dissolved branch that hook has
// for a MusicBrainz "Person"-type artist — #681's own field list only
// names founded_date/dissolved_date, so this form uses those regardless
// of artist_type.
const METADATA_KEYS = [
  'artist_type',
  'founded_date',
  'dissolved_date',
  'isni',
  'official_url',
  'wikipedia_url',
  'lastfm_url',
] as const

type MetadataKey = (typeof METADATA_KEYS)[number]

interface FormState {
  name: string
  sortName: string
  overview: string
  aliases: string[]
  metadata: Record<MetadataKey, string>
}

function metadataFromEntry(entry: LibraryEntry): Record<MetadataKey, string> {
  const result = {} as Record<MetadataKey, string>
  for (const key of METADATA_KEYS) {
    result[key] = stringField(entry.metadata, key) ?? ''
  }
  return result
}

function initialState(entry: LibraryEntry): FormState {
  return {
    name: entry.name,
    sortName: entry.sortName,
    overview: entry.overview,
    aliases: aliasesField(entry.metadata),
    metadata: metadataFromEntry(entry),
  }
}

function arraysEqual(a: string[], b: string[]): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

export interface EditArtistDialogProps {
  entry: LibraryEntry
  onClose: () => void
  onSaved: (entry: LibraryEntry) => void
}

// EditArtistDialog is #681's Artist manual field editor:
// name/sort_name/overview plus the Metadata identity facts (artist_type,
// aliases, founded/dissolved date, ISNI, official/Wikipedia/Last.fm URLs).
// UpdateLibraryEntry's field-mask only masks `metadata` atomically
// (applyLibraryEntryFieldMask has no per-sub-key path, same constraint
// useRefreshArtistMetadata.ts documents) — a touched metadata field patches
// a copy of entry.metadata so untouched keys (genre, style, country, the
// provider-fetched bio/backdrop caches) survive the round trip unchanged.
// Edit-only: unlike Person/Group, Artist has no manual-create dialog to
// share this form with (a LibraryEntry of kind Artist is only ever created
// via the MusicBrainz-backed Add Artist flow), so there's no `mode` prop.
// monitored/monitor_mode are excluded per #681 — ArtistDetail's own
// Monitor toggle already owns that control.
export function EditArtistDialog({ entry, onClose, onSaved }: EditArtistDialogProps) {
  const [initial] = useState(() => initialState(entry))
  const [form, setForm] = useState<FormState>(initial)
  const [nameTouched, setNameTouched] = useState(false)
  const mutation = useUpdateLibraryEntryMutation()

  const trimmedName = form.name.trim()
  const nameInvalid = trimmedName === ''

  const metadataTouched =
    !arraysEqual(form.aliases, initial.aliases) ||
    METADATA_KEYS.some(key => form.metadata[key] !== initial.metadata[key])
  const paths = [
    form.name !== initial.name && 'name',
    form.sortName !== initial.sortName && 'sort_name',
    form.overview !== initial.overview && 'overview',
    metadataTouched && 'metadata',
  ].filter((path): path is string => !!path)

  const canSubmit = !mutation.isPending && paths.length > 0

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  function updateMetadata(key: MetadataKey, value: string) {
    setForm(prev => ({ ...prev, metadata: { ...prev.metadata, [key]: value } }))
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setNameTouched(true)
    if (!canSubmit || nameInvalid) return

    const metadataPatch: JsonObject = { ...(entry.metadata ?? {}) }
    if (metadataTouched) {
      if (form.aliases.length > 0) metadataPatch.aliases = form.aliases
      else delete metadataPatch.aliases
      for (const key of METADATA_KEYS) {
        if (form.metadata[key] !== '') metadataPatch[key] = form.metadata[key]
        else delete metadataPatch[key]
      }
    }

    mutation.mutate(
      {
        libraryEntry: {
          id: entry.id,
          name: trimmedName,
          sortName: form.sortName,
          overview: form.overview,
          metadata: metadataTouched ? metadataPatch : undefined,
        },
        updateMask: { paths },
      },
      { onSuccess: response => response.libraryEntry && onSaved(response.libraryEntry) },
    )
  }

  return (
    <Modal title="Edit Artist" onClose={onClose}>
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

        <label className="flex flex-col gap-1 text-body text-text">
          <span className="text-label text-text-secondary">Overview</span>
          <textarea
            value={form.overview}
            onChange={e => update('overview', e.target.value)}
            rows={3}
            className="px-3 py-2 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          />
        </label>

        <ListInput label="Aliases" value={form.aliases} onChange={v => update('aliases', v)} />

        <TextInput
          label="Artist type"
          value={form.metadata.artist_type}
          onChange={v => updateMetadata('artist_type', String(v))}
        />

        <div className="flex gap-3">
          <TextInput
            label="Founded date"
            value={form.metadata.founded_date}
            onChange={v => updateMetadata('founded_date', String(v))}
          />
          <TextInput
            label="Dissolved date"
            value={form.metadata.dissolved_date}
            onChange={v => updateMetadata('dissolved_date', String(v))}
          />
        </div>

        <TextInput label="ISNI" value={form.metadata.isni} onChange={v => updateMetadata('isni', String(v))} />

        <TextInput
          label="Official site"
          value={form.metadata.official_url}
          onChange={v => updateMetadata('official_url', String(v))}
        />

        <TextInput
          label="Wikipedia"
          value={form.metadata.wikipedia_url}
          onChange={v => updateMetadata('wikipedia_url', String(v))}
        />

        <TextInput
          label="Last.fm"
          value={form.metadata.lastfm_url}
          onChange={v => updateMetadata('lastfm_url', String(v))}
        />

        {mutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't save this artist ({mutation.error.message}).
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
            {mutation.isPending ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
