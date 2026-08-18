import type { JsonObject } from '@bufbuild/protobuf'
import { Music, Plus, Search, SearchX } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AddArtistDialog } from '../components/AddArtistDialog'
import { ArtistCard } from '../components/ArtistCard'
import { EmptyState } from '../components/EmptyState'
import { Toggle } from '../components/Toggle'
import { useArtistLibraryEntries } from '../hooks/useArtistLibraryEntries'
import { useLibraryEntryImages } from '../hooks/useLibraryEntryImages'
import { useLibraryOwnership } from '../hooks/useLibraryOwnership'
import type { LibraryEntryRef } from '../types'

// genreOf reads LibraryEntry.Metadata["genre"] directly — populated at
// Add-Artist time by #665, zero additional fan-out needed here since
// it's already on the object ListLibraryEntries returns. Anything other
// than a plain string (unset, or some other JSON shape) renders no chip
// at all, per this issue's own acceptance criterion — never an empty
// placeholder.
function genreOf(metadata: JsonObject | undefined): string | undefined {
  const genre = metadata?.genre
  return typeof genre === 'string' && genre !== '' ? genre : undefined
}

// MusicLibrary — the Music Library page (#664).
// LibraryEntryService.ListLibraryEntries(kind="artist"), rendered as an
// ArtistCard (#658's Card config) grid. Both search and "Monitored only"
// filter the loaded page client-side — no server-side LibraryEntry
// search RPC exists or is being added by this story (see
// useArtistLibraryEntries), same accepted-limitation precedent the
// People index page (#660) set for its own monitored-only filter.
//
// Each card's ownership ring (#670) is a per-page fan-out via
// useLibraryOwnership — GroupService.ListGroups + per-group
// MusicReleaseService.ListMusicReleases, scoped to the visible artists on
// this page, never the whole library (see docs/technical/music-web-ui.md).
// Each card links to its Artist Detail route (#666) via a plain <Link>;
// ArtistCard itself has no nested interactive element (unlike
// PersonCard's photo button), so this needs none of People.tsx's
// click-target-detection workaround.
//
// The "Add Artist" button (#665) opens AddArtistDialog and navigates to
// the resulting artist's detail route (#666) on success.
export function MusicLibrary() {
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [monitoredOnly, setMonitoredOnly] = useState(false)
  const [addArtistOpen, setAddArtistOpen] = useState(false)

  const { data, isPending, isError, error, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useArtistLibraryEntries()

  const artists: LibraryEntryRef[] = useMemo(
    () =>
      (data?.pages.flatMap(page => page.libraryEntries) ?? []).map(entry => ({
        id: entry.id,
        name: entry.name,
        monitored: entry.monitored,
        genre: genreOf(entry.metadata),
      })),
    [data],
  )

  const query = search.trim().toLowerCase()
  const visibleArtists = useMemo(
    () =>
      artists
        .filter(artist => (monitoredOnly ? artist.monitored : true))
        .filter(artist => (query === '' ? true : artist.name.toLowerCase().includes(query))),
    [artists, monitoredOnly, query],
  )

  const artistIds = useMemo(() => visibleArtists.map(artist => artist.id), [visibleArtists])
  const imagesByArtistId = useLibraryEntryImages(artistIds)
  const ownershipByArtistId = useLibraryOwnership(artistIds)

  const isEmptyLibrary = !isPending && !isError && artists.length === 0 && query === '' && !monitoredOnly
  const isZeroResults = !isPending && !isError && artists.length > 0 && visibleArtists.length === 0

  return (
    <div className="px-6 py-10 md:px-8">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-headline text-text">Music Library</h1>

        <div className="flex flex-wrap items-center gap-3">
          <label className="relative">
            <Search
              size={16}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-text-secondary"
              aria-hidden="true"
            />
            <input
              type="text"
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search artists…"
              aria-label="Search artists"
              className="h-9 w-40 rounded-lg border border-border bg-surface-raised pl-8 pr-3 text-body text-text focus:outline-none focus:border-border-hover sm:w-52 md:w-64"
            />
          </label>

          <Toggle label="Monitored only" checked={monitoredOnly} onChange={setMonitoredOnly} />

          <button
            type="button"
            onClick={() => setAddArtistOpen(true)}
            className="flex h-9 items-center gap-1.5 rounded-lg bg-accent-system px-4 text-body font-medium text-bg hover:opacity-90"
          >
            <Plus size={16} aria-hidden="true" />
            Add Artist
          </button>
        </div>
      </div>

      {addArtistOpen && (
        <AddArtistDialog
          onClose={() => setAddArtistOpen(false)}
          onAdded={entry => {
            setAddArtistOpen(false)
            navigate(`/music/artists/${entry.id}`)
          }}
        />
      )}

      {isError && (
        <p className="mt-6 text-body text-status-failure">
          Couldn't load the Music Library ({error.message}).
        </p>
      )}

      {isEmptyLibrary && (
        <EmptyState
          icon={Music}
          title="No artists yet"
          description="Artists you add will show up here."
        />
      )}

      {isZeroResults && (
        <EmptyState
          icon={SearchX}
          title="No matches"
          description="No artists match your search or filter."
        />
      )}

      {visibleArtists.length > 0 && (
        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 2xl:grid-cols-10">
          {visibleArtists.map(artist => (
            <Link key={artist.id} to={`/music/artists/${artist.id}`} className="rounded-lg hover:bg-surface-raised">
              <ArtistCard
                artist={{ ...artist, imageId: imagesByArtistId[artist.id] }}
                ownership={ownershipByArtistId[artist.id]}
              />
            </Link>
          ))}
        </div>
      )}

      {hasNextPage && (
        <div className="mt-6 flex justify-center">
          <button
            type="button"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
            className="h-9 px-4 rounded-lg bg-surface-raised border border-border text-body font-medium text-text hover:bg-surface disabled:opacity-50"
          >
            {isFetchingNextPage ? 'Loading…' : 'Load more'}
          </button>
        </div>
      )}
    </div>
  )
}
