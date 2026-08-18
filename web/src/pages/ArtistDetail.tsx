import { useQuery } from '@connectrpc/connect-query'
import { Camera, Disc3, Images, Maximize2, Music, Plus, Users } from 'lucide-react'
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { AddAlbumDialog } from '../components/AddAlbumDialog'
import { AlbumCard } from '../components/AlbumCard'
import { ChooseArtworkDialog } from '../components/ChooseArtworkDialog'
import { DropdownMenu } from '../components/DropdownMenu'
import { EditActionButton } from '../components/EditActionButton'
import { EmptyState } from '../components/EmptyState'
import { Hero } from '../components/Hero'
import { ImageGallery } from '../components/ImageGallery'
import { ImageLightbox } from '../components/ImageLightbox'
import { ManualAlbumDialog } from '../components/ManualAlbumDialog'
import { PersonCard } from '../components/PersonCard'
import { RefreshArtistMetadataDialog } from '../components/RefreshArtistMetadataDialog'
import { Toggle } from '../components/Toggle'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
import { useArtistMembers } from '../hooks/useArtistMembers'
import { useArtistProviderData } from '../hooks/useArtistProviderData'
import { useDiscography } from '../hooks/useDiscography'
import { useGroupImages } from '../hooks/useGroupImages'
import { useLibraryEntry, useUpdateLibraryEntryMutation } from '../hooks/useLibraryEntry'
import { aliasesField, stringField } from '../lib/metadataFields'

// ArtistDetailTab — a local, state-driven tab registry (not route-based:
// unlike SettingsLayout's separately-routed tabs). Discography (#667) and
// Members (#668) share this one array/union rather than introducing a
// second tab mechanism.
type ArtistDetailTab = 'discography' | 'members'

const TAB_ITEMS: { id: ArtistDetailTab; label: string }[] = [
  { id: 'discography', label: 'Discography' },
  { id: 'members', label: 'Members' },
]

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
// EditActionButton (#671, module-wide "Edit ▾" convention per ADR 0004's
// `EditButton` vocabulary entry) sits in the Hero actions row: its primary
// "Edit" segment is disabled until #681 builds Artist's manual field
// editor; its chevron opens a provider-action menu, currently one entry —
// "Refresh from MusicBrainz" — disabled until provider.mbid is known
// (same gating "Add Album"'s MusicBrainz source uses), which opens
// RefreshArtistMetadataDialog. The dialog owns its own GetArtist(mbid)
// call and diff/apply logic (useRefreshArtistMetadata) — this page only
// refetches the LibraryEntry once the dialog reports a successful update.
//
// Discography tab (#667): GroupService.ListGroups(library_entry_id), then
// per group MusicReleaseService.ListMusicReleases(group_id) — see
// useDiscography — rendered as an AlbumCard (#658 Card config) grid. A
// Group with zero MusicRelease rows renders AlbumCard's own defined "No
// edition selected" badge rather than crashing on a missing default.
//
// "Add Album" (#669) is a DropdownMenu with two sources: "Search
// MusicBrainz" opens AddAlbumDialog, disabled until provider.mbid is
// known (ListReleaseGroupsForArtist has no non-MBID fallback, unlike the
// poster/backdrop buttons above); "Add Manually" opens ManualAlbumDialog
// for an album MusicBrainz doesn't have — a plain CreateGroup, no
// ExternalID (nothing to dedupe against without an external identity).
// Unlike Add Artist, picking a result never navigates — there is no
// Album Detail page yet (#673) — both paths just close their dialog and
// refetch this tab's own grid.
//
// Members tab (#668): EntryPersonService.ListEntryPeople(library_entry_id)
// — see useArtistMembers — rendered as two headed PersonCard (#657) grids,
// "Current members" and "Former members". "Former" is per-artist (a
// person can be current here and former on a different entry): a person
// lands in Former only when every role row they hold on *this* entry
// carries an EndDate. Each role chip carries its own era suffix
// (StartDate/EndDate) rather than PersonCard growing a dedicated field —
// see useArtistMembers.
export function ArtistDetail() {
  const { id = '' } = useParams<{ id: string }>()
  const entryQuery = useLibraryEntry(id)
  const updateMutation = useUpdateLibraryEntryMutation()
  const provider = useArtistProviderData(id)
  const discography = useDiscography(id)
  const albumImagesByGroupId = useGroupImages(discography.albums.map(album => album.id))
  const members = useArtistMembers(id)

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
  const [addAlbumOpen, setAddAlbumOpen] = useState(false)
  const [manualAlbumOpen, setManualAlbumOpen] = useState(false)
  const [refreshMetadataOpen, setRefreshMetadataOpen] = useState(false)

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

  const currentMembers = members.members.filter(member => !member.former)
  const formerMembers = members.members.filter(member => member.former)

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
            <EditActionButton
              editDisabledReason="Manual editing isn't available yet"
              items={[
                {
                  label: 'Refresh from MusicBrainz',
                  onSelect: () => setRefreshMetadataOpen(true),
                  disabled: !provider.mbid,
                },
              ]}
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
              <div className="mb-4 flex justify-end">
                <DropdownMenu
                  label="Add Album"
                  trigger={
                    <>
                      <Plus size={16} aria-hidden="true" />
                      Add Album
                    </>
                  }
                  triggerClassName="flex h-9 items-center gap-1.5 rounded-lg bg-accent-system px-4 text-body font-medium text-bg hover:opacity-90"
                  items={[
                    {
                      label: 'Search MusicBrainz',
                      onSelect: () => setAddAlbumOpen(true),
                      disabled: !provider.mbid,
                    },
                    { label: 'Add Manually', onSelect: () => setManualAlbumOpen(true) },
                  ]}
                />
              </div>

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

          {activeTab === 'members' && (
            <>
              {members.isError && (
                <p className="text-body text-status-failure" role="alert">
                  Couldn't load members.
                </p>
              )}

              {!members.isPending && !members.isError && members.members.length === 0 && (
                <EmptyState
                  icon={Users}
                  title="No members yet"
                  description="Band members added to this artist will show up here."
                />
              )}

              {currentMembers.length > 0 && (
                <div className="flex flex-col gap-3">
                  <h2 className="text-title-md font-semibold text-text">Current members</h2>
                  <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
                    {currentMembers.map(member => (
                      <PersonCard
                        key={member.personId}
                        person={{ id: member.personId, name: member.name, imageId: member.imageId }}
                        roles={member.roleLabels}
                      />
                    ))}
                  </div>
                </div>
              )}

              {formerMembers.length > 0 && (
                <div className="mt-6 flex flex-col gap-3">
                  <h2 className="text-title-md font-semibold text-text">Former members</h2>
                  <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
                    {formerMembers.map(member => (
                      <PersonCard
                        key={member.personId}
                        person={{ id: member.personId, name: member.name, imageId: member.imageId }}
                        roles={member.roleLabels}
                      />
                    ))}
                  </div>
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

      {addAlbumOpen && provider.mbid && (
        <AddAlbumDialog
          artistId={id}
          artistMbid={provider.mbid}
          onClose={() => setAddAlbumOpen(false)}
          onAdded={() => {
            setAddAlbumOpen(false)
            discography.refetch()
          }}
        />
      )}

      {manualAlbumOpen && (
        <ManualAlbumDialog
          artistId={id}
          onClose={() => setManualAlbumOpen(false)}
          onAdded={() => {
            setManualAlbumOpen(false)
            discography.refetch()
          }}
        />
      )}

      {refreshMetadataOpen && provider.mbid && (
        <RefreshArtistMetadataDialog
          entry={entry}
          mbid={provider.mbid}
          onClose={() => setRefreshMetadataOpen(false)}
          onUpdated={() => {
            setRefreshMetadataOpen(false)
            entryQuery.refetch()
          }}
        />
      )}
    </div>
  )
}
