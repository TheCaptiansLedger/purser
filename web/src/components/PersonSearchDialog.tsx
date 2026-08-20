import { useQuery } from '@connectrpc/connect-query'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { Search, SearchX } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { searchArtists } from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import type { MusicBrainzArtist } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { useGetOrCreatePersonFromMusicBrainz } from '../hooks/useGetOrCreatePersonFromMusicBrainz'
import { EmptyState } from './EmptyState'
import { Modal } from './Modal'

const MIN_QUERY_LENGTH = 2
const DEBOUNCE_MS = 400

// useDebouncedValue delays reflecting value until it's stopped changing for
// delayMs — same local copy AddArtistDialog's own uses; extract to a
// shared hook if a third search-as-you-type consumer shows up.
function useDebouncedValue(value: string, delayMs: number): string {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])
  return debounced
}

// personSummary joins the row's secondary details (country, life span)
// with a separator, skipping any that are empty rather than rendering a
// stray " · ". No "type" segment here, unlike AddArtistDialog's
// artistSummary — every row in this list is already known to be a
// Person.
function personSummary(artist: MusicBrainzArtist): string {
  const lifeSpan = [artist.lifeSpanBegin, artist.lifeSpanEnd].filter(Boolean).join('–')
  return [artist.country, lifeSpan].filter(Boolean).join(' · ')
}

// mbzDate converts a MusicBrainz life-span string ("1940", "1940-05",
// "1940-05-27") into a Timestamp — same partial-date handling
// useAddArtist's musicBrainzMemberDate already uses. Empty string
// (MusicBrainz's own "unknown" shape) returns undefined.
function mbzDate(value: string) {
  if (value === '') return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : timestampFromDate(date)
}

export interface PersonSearchDialogProps {
  onClose: () => void
  // onAdded fires once the get-or-create composition resolves to a final
  // Person id (whether newly created or already existing) — the caller
  // decides what "done" means (navigate, close, refresh), same contract
  // AddArtistDialog's onAdded uses.
  onAdded: (personId: string) => void
}

// PersonSearchDialog is #723's standalone "Search MusicBrainz" entry
// point for Add Person: free-text search over MusicBrainzService
// .SearchArtists (#649, the same RPC AddArtistDialog already uses),
// results filtered client-side to type=="Person" — a "Group" result
// makes no sense as a Person candidate, so it's simply not shown rather
// than shown-and-disabled. Picking a result runs
// useGetOrCreatePersonFromMusicBrainz's ADR 0026 composition. This
// dialog is presentation only, same split AddArtistDialog/useAddArtist
// already established.
export function PersonSearchDialog({ onClose, onAdded }: PersonSearchDialogProps) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebouncedValue(query, DEBOUNCE_MS).trim()
  const { getOrCreatePerson } = useGetOrCreatePersonFromMusicBrainz()
  const [addingMbid, setAddingMbid] = useState<string | null>(null)
  const [addError, setAddError] = useState<string | null>(null)

  const { data, isPending, isError, error } = useQuery(
    searchArtists,
    { query: debouncedQuery },
    { enabled: debouncedQuery.length >= MIN_QUERY_LENGTH },
  )
  const people = useMemo(() => (data?.artists ?? []).filter(artist => artist.type === 'Person'), [data])

  async function handleSelect(candidate: MusicBrainzArtist) {
    setAddError(null)
    setAddingMbid(candidate.mbid)
    try {
      const personId = await getOrCreatePerson({
        mbid: candidate.mbid,
        name: candidate.name,
        sortName: candidate.sortName,
        nationality: candidate.country,
        birthDate: mbzDate(candidate.lifeSpanBegin),
        deathDate: mbzDate(candidate.lifeSpanEnd),
      })
      onAdded(personId)
    } catch (err) {
      setAddError(err instanceof Error ? err.message : 'Could not add this person.')
    } finally {
      setAddingMbid(null)
    }
  }

  return (
    <Modal title="Search MusicBrainz" onClose={onClose}>
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
            placeholder="Search MusicBrainz for a person…"
            aria-label="Search MusicBrainz for a person"
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

        {!isPending && !isError && debouncedQuery.length >= MIN_QUERY_LENGTH && people.length === 0 && (
          <EmptyState icon={SearchX} title="No matches" description="No MusicBrainz people match your search." />
        )}

        {people.length > 0 && (
          <ul className="flex max-h-96 flex-col gap-1 overflow-y-auto">
            {people.map(person => (
              <li key={person.mbid}>
                <button
                  type="button"
                  onClick={() => handleSelect(person)}
                  disabled={addingMbid !== null}
                  className="flex w-full flex-col items-start gap-0.5 rounded-lg px-3 py-2 text-left hover:bg-surface disabled:opacity-50"
                >
                  <span className="text-body font-medium text-text">
                    {person.name}
                    {person.disambiguation !== '' && (
                      <span className="ml-2 text-label text-text-secondary">{person.disambiguation}</span>
                    )}
                  </span>
                  <span className="text-label text-text-secondary">
                    {personSummary(person)}
                    {addingMbid === person.mbid && ' · Adding…'}
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
