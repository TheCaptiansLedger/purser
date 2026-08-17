import type { JsonObject } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { Camera, Disc3, Images, Maximize2, Music } from 'lucide-react'
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { AlbumCard } from '../components/AlbumCard'
import { ChooseArtworkDialog } from '../components/ChooseArtworkDialog'
import { EmptyState } from '../components/EmptyState'
import { Hero } from '../components/Hero'
import { ImageGallery } from '../components/ImageGallery'
import { ImageLightbox } from '../components/ImageLightbox'
import { Toggle } from '../components/Toggle'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
import { useArtistProviderData } from '../hooks/useArtistProviderData'
import { useDiscography } from '../hooks/useDiscography'
import { useGroupImages } from '../hooks/useGroupImages'
import { useLibraryEntry, useUpdateLibraryEntryMutation } from '../hooks/useLibraryEntry'

// ArtistDetailTab — a local, state-driven tab registry (not route-based:
// unlike SettingsLayout's separately-routed tabs, #668's future "Members"
// tab lives on this same /music/artists/:id route). Discography (#667)
// is the only working tab today; #668 extends this exact array/union
// rather than introducing a second tab mechanism.
type ArtistDetailTab = 'discography'

const TAB_ITEMS: { id: ArtistDetailTab; label: string }[] = [{ id: 'discography', label: 'Discography' }]

function stringField(metadata: JsonObject | undefined, key: string): string | undefined {
  const value = metadata?.[key]
  return typeof value === 'string' && value !== '' ? value : undefined
}

function aliasesField(metadata: JsonObject | undefined): string[] {
  const value = metadata?.aliases
  return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string') : []
}

// ArtistDetail — the Artist Detail page shell (#666). LibraryEntryService
// .GetLibraryEntry, a Hero (#658) whose backdrop comes from fanart.tv's
// artist_background (falling back to the user's own selected backdrop
// image once attached), a facts sidebar built from LibraryEntry.Metadata
// (artist_type, country, founded/dissolved or born/died dates, ISNI,
// aliases, official-site/Wikipedia links — the last four written once at
// Add Artist time by useAddArtist's MusicBrainz-relations step, not
// re-fetched live here; structured identity facts persist into Metadata
// the same way artist_type/aliases already do, unlike the bio/backdrop
// below which are fetched fresh per ADR 0027), a bio panel from
// TheAudioDB's biography, and the Monitor toggle (UpdateLibraryEntry
// field-mask [monitored, monitor_mode]).
//
// Poster and backdrop both attach through #656's ChooseArtworkDialog
// (owner_type="library_entry", distinguished by image_type) and view
// through #655's ImageLightbox. Poster offers fanart.tv's artist_thumb as
// candidates; backdrop offers artist_background and hd_music_logo —
// each fanart.tv field feeds exactly one slot, presented side-by-side
// per ADR 0027 rather than the server picking one. Both dialogs also
// fall back to a plain upload when no candidates exist.
//
// No `mbz` ExternalID row, or a 404 from either provider, degrades to
// that section simply being absent (see useArtistProviderData) — this
// page never errors out over missing provider data, only over a missing
// LibraryEntry itself.
//
// Discography tab (#667): GroupService.ListGroups(library_entry_id), then
// per group MusicReleaseService.ListMusicReleases(group_id) — see
// useDiscography — rendered as an AlbumCard (#658 Card config) grid. A
// Group with zero MusicRelease rows renders AlbumCard's own defined "No
// edition selected" badge rather than crashing on a missing default.
export function ArtistDetail() {
  const { id = '' } = useParams<{ id: string }>()
  const entryQuery = useLibraryEntry(id)
  const updateMutation = useUpdateLibraryEntryMutation()
  const provider = useArtistProviderData(id)
  const discography = useDiscography(id)
  const albumImagesByGroupId = useGroupImages(discography.albums.map(album => album.id))

  const posterQuery = useQuery(
    getSelectedImage,
    { ownerType: 'library_entry', ownerId: id, imageType: 'poster' },
    { enabled: !!id, retry: false },
  )
  const backdropQuery = useQuery(
    getSelectedImage,
    { ownerType: 'library_entry', ownerId: id, imageType: 'backdrop' },
    { enabled: !!id, retry: false },
  )

  // Optimistic local mirror of the one server-owned field this page can
  // write directly — same pattern as PersonDetail's monitored state.
  const [monitored, setMonitored] = useState<boolean | undefined>(undefined)
  const [posterLightboxOpen, setPosterLightboxOpen] = useState(false)
  const [posterDialogOpen, setPosterDialogOpen] = useState(false)
  const [posterGalleryOpen, setPosterGalleryOpen] = useState(false)
  const [backdropLightboxOpen, setBackdropLightboxOpen] = useState(false)
  const [backdropDialogOpen, setBackdropDialogOpen] = useState(false)
  const [backdropGalleryOpen, setBackdropGalleryOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<ArtistDetailTab>('discography')

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
  if (entryQuery.isPending) {
    return null
  }

  if (entryQuery.isError) {
    return (
      <p className="mx-6 mt-10 text-body text-status-failure" role="alert">
        Couldn't load this artist ({entryQuery.error.message}).
      </p>
    )
  }

  const entry = entryQuery.data.libraryEntry
  if (!entry) {
    return (
      <p className="mx-6 mt-10 text-body text-status-failure" role="alert">
        Artist not found.
      </p>
    )
  }

  const isMonitored = monitored ?? entry.monitored

  const posterImageId = posterQuery.data?.image?.id
  const posterSrc = posterImageId ? `/media/images/${posterImageId}` : undefined

  const backdropImageId = backdropQuery.data?.image?.id
  const selectedBackdropSrc = backdropImageId ? `/media/images/${backdropImageId}` : undefined
  const backdropSrc = selectedBackdropSrc ?? provider.backdropUrl

  function handleMonitorToggle(next: boolean) {
    const previous = isMonitored
    setMonitored(next)
    updateMutation.mutate(
      {
        libraryEntry: { id, monitored: next, monitorMode: next ? MonitorMode.ALL : MonitorMode.NONE },
        updateMask: { paths: ['monitored', 'monitor_mode'] },
      },
      {
        onSuccess: response => setMonitored(response.libraryEntry?.monitored ?? next),
        onError: () => setMonitored(previous),
      },
    )
  }

  const metadata = entry.metadata
  const genre = stringField(metadata, 'genre')
  const style = stringField(metadata, 'style')
  const heroFacts = [genre, style].filter((fact): fact is string => !!fact)

  const artistType = stringField(metadata, 'artist_type')
  const isPerson = artistType === 'Person'
  const startDate = stringField(metadata, isPerson ? 'born_date' : 'founded_date')
  const endDate = stringField(metadata, isPerson ? 'died_date' : 'dissolved_date')
  const aliases = aliasesField(metadata)
  const country = stringField(metadata, 'country')
  const isni = stringField(metadata, 'isni')
  const officialUrl = stringField(metadata, 'official_url')
  const wikipediaUrl = stringField(metadata, 'wikipedia_url')

  const facts = [
    artistType,
    country,
    startDate && endDate ? `${startDate}–${endDate}` : startDate && `Since ${startDate}`,
    isni && `ISNI ${isni}`,
    aliases.length > 0 && `Also known as ${aliases.join(', ')}`,
  ].filter((fact): fact is string => !!fact)

  return (
    <div className="px-6 py-10 md:px-8">
      <Hero
        backdropSrc={backdropSrc}
        title={entry.name}
        facts={heroFacts}
        actions={
          <>
            <Toggle
              label="Monitored"
              checked={isMonitored}
              onChange={handleMonitorToggle}
              disabled={updateMutation.isPending}
            />
            <button
              type="button"
              onClick={() => setBackdropDialogOpen(true)}
              className="flex h-8 items-center gap-2 rounded-lg px-3 text-label font-medium text-text-secondary hover:bg-surface-raised hover:text-text"
            >
              <Camera size={14} />
              {selectedBackdropSrc ? 'Change backdrop' : 'Add backdrop'}
            </button>
            <button
              type="button"
              onClick={() => setBackdropGalleryOpen(true)}
              aria-label="Manage backdrops"
              title="Manage backdrops"
              className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text"
            >
              <Images size={14} />
            </button>
            {selectedBackdropSrc && (
              <button
                type="button"
                onClick={() => setBackdropLightboxOpen(true)}
                aria-label="View backdrop full-screen"
                title="View backdrop full-screen"
                className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text"
              >
                <Maximize2 size={14} />
              </button>
            )}
          </>
        }
      />

      <div className="mt-6 flex flex-col gap-6 sm:flex-row sm:items-start">
        <div className="flex flex-col items-center gap-2 sm:w-48 shrink-0">
          {posterSrc ? (
            <button
              type="button"
              onClick={() => setPosterLightboxOpen(true)}
              aria-label={`View ${entry.name}'s poster`}
              className="aspect-[2/3] w-full overflow-hidden rounded-xl border border-border"
            >
              <img src={posterSrc} alt={entry.name} className="h-full w-full object-cover" />
            </button>
          ) : (
            <div
              aria-hidden="true"
              className="flex aspect-[2/3] w-full items-center justify-center rounded-xl border border-border bg-surface-raised text-text-secondary"
            >
              <Music size={40} />
            </div>
          )}

          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setPosterDialogOpen(true)}
              className="flex h-8 items-center gap-2 rounded-lg px-3 text-label font-medium text-text-secondary hover:bg-surface-raised hover:text-text"
            >
              <Camera size={14} />
              {posterSrc ? 'Change poster' : 'Add poster'}
            </button>

            <button
              type="button"
              onClick={() => setPosterGalleryOpen(true)}
              aria-label="Manage posters"
              title="Manage posters"
              className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text"
            >
              <Images size={14} />
            </button>
          </div>
        </div>

        <div className="flex flex-1 flex-col gap-4">
          {(facts.length > 0 || officialUrl || wikipediaUrl) && (
            <div className="flex max-w-sm flex-col gap-2 rounded-xl border border-border bg-surface p-4">
              {facts.map(fact => (
                <span key={fact} className="text-body text-text-secondary">
                  {fact}
                </span>
              ))}
              {officialUrl && (
                <a href={officialUrl} target="_blank" rel="noreferrer" className="text-body font-medium text-accent-system hover:underline">
                  Official site
                </a>
              )}
              {wikipediaUrl && (
                <a href={wikipediaUrl} target="_blank" rel="noreferrer" className="text-body font-medium text-accent-system hover:underline">
                  Wikipedia
                </a>
              )}
            </div>
          )}

          {provider.bio && <p className="max-w-2xl text-body text-text-secondary">{provider.bio}</p>}
        </div>
      </div>

      <div className="mt-8">
        <div role="tablist" aria-label="Artist detail" className="flex gap-1 border-b border-border">
          {TAB_ITEMS.map(tab => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={activeTab === tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={[
                'px-3 h-10 flex items-center text-body border-b-2 -mb-px transition-colors',
                'text-text-secondary hover:text-text',
                activeTab === tab.id ? 'text-text border-text' : 'border-transparent',
              ].join(' ')}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div role="tabpanel" className="mt-6">
          {activeTab === 'discography' && (
            <>
              {!discography.isPending && discography.albums.length === 0 && (
                <EmptyState
                  icon={Disc3}
                  title="No albums yet"
                  description="Albums added to this artist will show up here."
                />
              )}

              {discography.albums.length > 0 && (
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
                  {discography.albums.map(album => (
                    <AlbumCard key={album.id} album={{ ...album, imageId: albumImagesByGroupId[album.id] }} />
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {posterLightboxOpen && posterSrc && (
        <ImageLightbox src={posterSrc} alt={entry.name} onClose={() => setPosterLightboxOpen(false)} />
      )}

      {posterDialogOpen && (
        <ChooseArtworkDialog
          title={`${posterSrc ? 'Change' : 'Add'} poster`}
          ownerType="library_entry"
          ownerId={id}
          imageType="poster"
          candidates={provider.posterCandidates}
          onClose={() => setPosterDialogOpen(false)}
          onAttached={() => void posterQuery.refetch()}
        />
      )}

      {posterGalleryOpen && (
        <ImageGallery
          title="Manage posters"
          ownerType="library_entry"
          ownerId={id}
          imageType="poster"
          onClose={() => setPosterGalleryOpen(false)}
          onChange={() => void posterQuery.refetch()}
        />
      )}

      {backdropLightboxOpen && selectedBackdropSrc && (
        <ImageLightbox src={selectedBackdropSrc} alt={`${entry.name} backdrop`} onClose={() => setBackdropLightboxOpen(false)} />
      )}

      {backdropDialogOpen && (
        <ChooseArtworkDialog
          title={`${selectedBackdropSrc ? 'Change' : 'Add'} backdrop`}
          ownerType="library_entry"
          ownerId={id}
          imageType="backdrop"
          candidates={provider.backdropCandidates}
          onClose={() => setBackdropDialogOpen(false)}
          onAttached={() => void backdropQuery.refetch()}
        />
      )}

      {backdropGalleryOpen && (
        <ImageGallery
          title="Manage backdrops"
          ownerType="library_entry"
          ownerId={id}
          imageType="backdrop"
          onClose={() => setBackdropGalleryOpen(false)}
          onChange={() => void backdropQuery.refetch()}
        />
      )}
    </div>
  )
}
