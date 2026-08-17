import { useQuery } from '@connectrpc/connect-query'
import { Search, SearchX } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { searchArtists } from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import type { MusicBrainzArtist } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { useAddArtist } from '../hooks/useAddArtist'
import { EmptyState } from './EmptyState'
import { Modal } from './Modal'

const MIN_QUERY_LENGTH = 2
const DEBOUNCE_MS = 400

// useDebouncedValue delays reflecting value until it's stopped changing for
// delayMs — kept local to this dialog since it's the only search-as-you-type
// consumer in the codebase so far; extract if a second one shows up.
function useDebouncedValue(value: string, delayMs: number): string {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])
  return debounced
}

// artistSummary joins the row's secondary details (type, country, life
// span) with a separator, skipping any that are empty rather than
// rendering a stray " · ".
function artistSummary(artist: MusicBrainzArtist): string {
  const lifeSpan = [artist.lifeSpanBegin, artist.lifeSpanEnd].filter(Boolean).join('–')
  return [artist.type, artist.country, lifeSpan].filter(Boolean).join(' · ')
}

export interface AddArtistDialogProps {
  onClose: () => void
  // onAdded fires once the get-or-create composition resolves to a final
  // LibraryEntry (whether newly created or already existing) — the
  // caller decides what "done" means (navigate, close, refresh).
  onAdded: (entry: LibraryEntry) => void
}

// AddArtistDialog is #665's Add Artist flow: free-text search over
// MusicBrainzService.SearchArtists, then useAddArtist's ADR 0026
// get-or-create composition once a result is picked. This dialog is
// presentation only — the composition logic lives entirely in
// useAddArtist so it can be unit tested independent of any rendering.
export function AddArtistDialog({ onClose, onAdded }: AddArtistDialogProps) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebouncedValue(query, DEBOUNCE_MS).trim()
  const { addArtist } = useAddArtist()
  const [addingMbid, setAddingMbid] = useState<string | null>(null)
  const [addError, setAddError] = useState<string | null>(null)

  const { data, isPending, isError, error } = useQuery(
    searchArtists,
    { query: debouncedQuery },
    { enabled: debouncedQuery.length >= MIN_QUERY_LENGTH },
  )
  const artists = data?.artists ?? []

  async function handleSelect(candidate: MusicBrainzArtist) {
    setAddError(null)
    setAddingMbid(candidate.mbid)
    try {
      const entry = await addArtist(candidate)
      onAdded(entry)
    } catch (err) {
      setAddError(err instanceof Error ? err.message : 'Could not add this artist.')
    } finally {
      setAddingMbid(null)
    }
  }

  return (
    <Modal title="Add Artist" onClose={onClose}>
      <div className="flex flex-col gap-4">
        <label className="relative">
          <Search
            size={16}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-text-secondary"
            aria-hidden="true"
          />
          <input
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search MusicBrainz for an artist…"
            aria-label="Search MusicBrainz for an artist"
            autoFocus
            className="h-9 w-full rounded-lg border border-border bg-surface-raised pl-8 pr-3 text-body text-text focus:outline-none focus:border-border-hover"
          />
        </label>

        {isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't search MusicBrainz ({error.message}).
          </p>
        )}

        {isPending && debouncedQuery.length >= MIN_QUERY_LENGTH && (
          <p className="text-body text-text-secondary">Searching…</p>
        )}

        {!isPending && !isError && debouncedQuery.length >= MIN_QUERY_LENGTH && artists.length === 0 && (
          <EmptyState icon={SearchX} title="No matches" description="No MusicBrainz artists match your search." />
        )}

        {artists.length > 0 && (
          <ul className="flex max-h-96 flex-col gap-1 overflow-y-auto">
            {artists.map(artist => (
              <li key={artist.mbid}>
                <button
                  type="button"
                  onClick={() => handleSelect(artist)}
                  disabled={addingMbid !== null}
                  className="flex w-full flex-col items-start gap-0.5 rounded-lg px-3 py-2 text-left hover:bg-surface disabled:opacity-50"
                >
                  <span className="text-body font-medium text-text">
                    {artist.name}
                    {artist.disambiguation !== '' && (
                      <span className="ml-2 text-label text-text-secondary">{artist.disambiguation}</span>
                    )}
                  </span>
                  <span className="text-label text-text-secondary">
                    {artistSummary(artist)}
                    {addingMbid === artist.mbid && ' · Adding…'}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}

        {addError && (
          <p className="text-body text-status-failure" role="alert">
            {addError}
          </p>
        )}
      </div>
    </Modal>
  )
}
