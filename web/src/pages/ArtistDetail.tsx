import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Camera, CheckSquare, Disc3, Images, Maximize2, Music, Plus, Users } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { AddAlbumDialog } from '../components/AddAlbumDialog'
import { AddMemberDialog } from '../components/AddMemberDialog'
import { AlbumCard } from '../components/AlbumCard'
import { BulkActionErrors, type BulkActionError } from '../components/BulkActionErrors'
import { BulkDeleteDialog } from '../components/BulkDeleteDialog'
import { ChooseArtworkDialog } from '../components/ChooseArtworkDialog'
import { DropdownMenu } from '../components/DropdownMenu'
import { EditActionButton } from '../components/EditActionButton'
import { EditArtistDialog } from '../components/EditArtistDialog'
import { EditMemberDialog } from '../components/EditMemberDialog'
import { EmptyState } from '../components/EmptyState'
import { GroupDialog } from '../components/GroupDialog'
import { Hero } from '../components/Hero'
import { ImageGallery } from '../components/ImageGallery'
import { ImageLightbox } from '../components/ImageLightbox'
import { PersonCard, type PersonCardRole } from '../components/PersonCard'
import { RefreshArtistMetadataDialog } from '../components/RefreshArtistMetadataDialog'
import { RemoveMemberDialog } from '../components/RemoveMemberDialog'
import { SelectableTile } from '../components/SelectableTile'
import { SelectionToolbar } from '../components/SelectionToolbar'
import { Toggle } from '../components/Toggle'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { bulkDeleteGroups, updateGroup } from '../gen/purser/domain/v1/group-GroupService_connectquery'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
import { useArtistMembers, type ArtistMember } from '../hooks/useArtistMembers'
import { useArtistProviderData } from '../hooks/useArtistProviderData'
import { useDiscography } from '../hooks/useDiscography'
import { useGroupDeletionImpacts } from '../hooks/useGroupDeletionImpacts'
import { useGroupImages } from '../hooks/useGroupImages'
import { useLibraryEntry, useUpdateLibraryEntryMutation } from '../hooks/useLibraryEntry'
import { aliasesField, stringField } from '../lib/metadataFields'

// ArtistDetailTab — a local, state-driven tab registry (not route-based:
// unlike SettingsLayout's separately-routed tabs). Discography (#667) and
// Members (#668) share this one array/union rather than introducing a
// second tab mechanism.
type ArtistDetailTab = 'discography' | 'members'

// MemberRow is the Edit/Remove dialogs' own working set — a single
// EntryPerson row's identity (personId + role) plus era, and the display
// name for the confirm/dialog copy. EntryPerson's identity is
// (library_entry_id, person_id, role), not person_id alone, so this stays
// per-role even though the grid below renders per-person.
interface MemberRow {
  personId: string
  name: string
  role: string
  startDate?: ArtistMember['roles'][number]['startDate']
  endDate?: ArtistMember['roles'][number]['endDate']
}

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
// "Edit" segment opens EditArtistDialog (#681 — name/sort_name/overview
// plus the identity-fact Metadata keys), its chevron opens a provider-
// action menu, currently one entry — "Refresh from MusicBrainz" —
// disabled until provider.mbid is known (same gating "Add Album"'s
// MusicBrainz source uses), which opens RefreshArtistMetadataDialog. That
// dialog owns its own GetArtist(mbid) call and diff/apply logic
// (useRefreshArtistMetadata) — this page only refetches the LibraryEntry
// once either dialog reports a successful update.
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
// poster/backdrop buttons above); "Add Manually" opens GroupDialog in
// create mode for an album MusicBrainz doesn't have — a plain CreateGroup,
// no ExternalID (nothing to dedupe against without an external identity).
// Unlike Add Artist, picking a result never navigates — both paths just
// close their dialog and refetch this tab's own grid; the new album is
// reached afterward the same way any other one is, by clicking its
// AlbumCard through to Album Detail (#673).
//
// Bulk delete (#679): "Select" swaps the grid's <Link> wrap for
// SelectableTile's toggle-button one, exposing a SelectionToolbar; its
// "Delete" opens BulkDeleteDialog against
// useGroupDeletionImpacts(selectedAlbumIds) and BulkDeleteGroups — see
// docs/adr/0015/0016. Group never blocks a delete
// (internal/service/group_deletion.go), so unlike the Library grid's
// artist delete, the dialog's cascade checkbox never actually appears
// here.
//
// Bulk monitor toggle (#680): same client-side Promise.allSettled loop as
// the Library grid's own bulk monitor toggle, here over the existing
// single-row UpdateGroup RPC ([monitored, monitor_mode] field mask) — see
// MusicLibrary.tsx's own comment for the ADR 0016 justification. Per-row
// failures render via BulkActionErrors; the tab refetches once the loop
// settles.
//
// Members tab (#668, #724): EntryPersonService.ListEntryPeople
// (library_entry_id) — see useArtistMembers — rendered as two headed
// PersonCard (#657) grids, "Current members" and "Former members", same
// merged-card-per-person shape #668 shipped. "Former" is per-artist (a
// person can be current here and former on a different entry): a person
// lands in Former only when every role row they hold on *this* entry
// carries an EndDate. Each role chip carries its own era suffix
// (StartDate/EndDate) — see useArtistMembers.
//
// #724 turns each chip actionable: PersonCard's `roles` now carries
// per-chip onEdit/onRemove callbacks (PersonCardRole), keyed by the raw
// EntryPerson `role` string rather than the chip's own formatted label,
// since Edit/Remove act on one EntryPerson row (person_id, role), not the
// merged card. "Add member" opens AddMemberDialog: search existing People
// (ListPeople name filter) or create a new Person (#663's PersonDialog,
// reused as a sequential step per this issue's own scope decision), then
// a role/era step that calls CreateEntryPerson. A chip's Edit icon opens
// EditMemberDialog (role/era, pre-filled); Remove opens
// RemoveMemberDialog, a plain confirm — EntryPerson is a pure join row
// with no referrers of its own, so no deletion-impact preview applies
// here (ADR 0015's own scope note). All three refetch this tab's own list
// on success.
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
  const [editArtistOpen, setEditArtistOpen] = useState(false)
  const [selectMode, setSelectMode] = useState(false)
  const [selectedAlbumIds, setSelectedAlbumIds] = useState<Set<string>>(new Set())
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false)
  const [albumMonitorErrors, setAlbumMonitorErrors] = useState<BulkActionError[]>([])
  const [addMemberOpen, setAddMemberOpen] = useState(false)
  const [editingMemberRow, setEditingMemberRow] = useState<MemberRow | undefined>(undefined)
  const [removingMemberRow, setRemovingMemberRow] = useState<MemberRow | undefined>(undefined)

  const selectedAlbumIdList = useMemo(() => Array.from(selectedAlbumIds), [selectedAlbumIds])
  const albumImpact = useGroupDeletionImpacts(bulkDeleteOpen ? selectedAlbumIdList : [])
  const bulkDeleteAlbumsMutation = useMutation(bulkDeleteGroups)
  const updateGroupMutation = useMutation(updateGroup)

  function toggleAlbumSelected(albumId: string) {
    setSelectedAlbumIds(prev => {
      const next = new Set(prev)
      if (next.has(albumId)) {
        next.delete(albumId)
      } else {
        next.add(albumId)
      }
      return next
    })
  }

  function exitAlbumSelectMode() {
    setSelectMode(false)
    setSelectedAlbumIds(new Set())
    setAlbumMonitorErrors([])
  }

  async function handleBulkMonitorAlbums(monitored: boolean) {
    setAlbumMonitorErrors([])
    const targets = discography.albums.filter(album => selectedAlbumIds.has(album.id))
    const results = await Promise.allSettled(
      targets.map(album =>
        updateGroupMutation.mutateAsync({
          group: { id: album.id, monitored, monitorMode: monitored ? MonitorMode.ALL : MonitorMode.NONE },
          updateMask: { paths: ['monitored', 'monitor_mode'] },
        }),
      ),
    )
    setAlbumMonitorErrors(
      results.flatMap((result, index) =>
        result.status === 'rejected'
          ? [{ id: targets[index].id, label: targets[index].title, message: (result.reason as Error).message }]
          : [],
      ),
    )
    discography.refetch()
  }

  function handleBulkDeleteAlbums(cascade: boolean) {
    bulkDeleteAlbumsMutation.mutate(
      { ids: selectedAlbumIdList, cascade },
      {
        onSuccess: () => {
          setBulkDeleteOpen(false)
          exitAlbumSelectMode()
          discography.refetch()
        },
      },
    )
  }

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

  // memberCardRoles maps one ArtistMember's roles onto PersonCard's chip
  // shape, wiring each chip's Edit/Remove icons to that specific
  // EntryPerson row (personId + raw role, not the merged card).
  function memberCardRoles(member: ArtistMember): PersonCardRole[] {
    return member.roles.map(role => ({
      label: role.label,
      id: role.role,
      onEdit: () =>
        setEditingMemberRow({
          personId: member.personId,
          name: member.name,
          role: role.role,
          startDate: role.startDate,
          endDate: role.endDate,
        }),
      onRemove: () => setRemovingMemberRow({ personId: member.personId, name: member.name, role: role.role }),
    }))
  }

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
              onEdit={() => setEditArtistOpen(true)}
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
              <div className="mb-4 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setSelectMode(true)}
                  className="flex h-9 items-center gap-1.5 rounded-lg border border-border px-4 text-body font-medium text-text hover:bg-surface-raised"
                >
                  <CheckSquare size={16} aria-hidden="true" />
                  Select
                </button>

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

              {selectMode && (
                <SelectionToolbar
                  count={selectedAlbumIds.size}
                  entityLabelPlural="albums"
                  onDelete={() => setBulkDeleteOpen(true)}
                  onCancel={exitAlbumSelectMode}
                  onMonitor={() => void handleBulkMonitorAlbums(true)}
                  onUnmonitor={() => void handleBulkMonitorAlbums(false)}
                  isUpdatingMonitored={updateGroupMutation.isPending}
                />
              )}

              {selectMode && <BulkActionErrors errors={albumMonitorErrors} />}

              {!discography.isPending && discography.albums.length === 0 && (
                <EmptyState
                  icon={Disc3}
                  title="No albums yet"
                  description="Albums added to this artist will show up here."
                />
              )}

              {discography.albums.length > 0 && (
                <div className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
                  {discography.albums.map(album => (
                    <SelectableTile
                      key={album.id}
                      to={`/music/albums/${album.id}`}
                      selectMode={selectMode}
                      selected={selectedAlbumIds.has(album.id)}
                      onToggle={() => toggleAlbumSelected(album.id)}
                    >
                      <AlbumCard album={{ ...album, imageId: albumImagesByGroupId[album.id] }} />
                    </SelectableTile>
                  ))}
                </div>
              )}

              {bulkDeleteOpen && (
                <BulkDeleteDialog
                  entityLabelPlural="albums"
                  count={selectedAlbumIds.size}
                  impact={albumImpact}
                  onDelete={handleBulkDeleteAlbums}
                  isDeleting={bulkDeleteAlbumsMutation.isPending}
                  deleteError={
                    bulkDeleteAlbumsMutation.isError
                      ? `Couldn't delete these albums (${bulkDeleteAlbumsMutation.error.message}).`
                      : undefined
                  }
                  onClose={() => setBulkDeleteOpen(false)}
                />
              )}
            </>
          )}

          {activeTab === 'members' && (
            <>
              <div className="mb-4 flex justify-end">
                <button
                  type="button"
                  onClick={() => setAddMemberOpen(true)}
                  className="flex h-9 items-center gap-1.5 rounded-lg bg-accent-system px-4 text-body font-medium text-bg hover:opacity-90"
                >
                  <Plus size={16} aria-hidden="true" />
                  Add member
                </button>
              </div>

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
                        roles={memberCardRoles(member)}
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
                        roles={memberCardRoles(member)}
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
        <GroupDialog
          mode="create"
          artistId={id}
          onClose={() => setManualAlbumOpen(false)}
          onSaved={() => {
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

      {editArtistOpen && (
        <EditArtistDialog
          entry={entry}
          onClose={() => setEditArtistOpen(false)}
          onSaved={() => {
            setEditArtistOpen(false)
            entryQuery.refetch()
          }}
        />
      )}

      {addMemberOpen && (
        <AddMemberDialog
          libraryEntryId={id}
          onClose={() => setAddMemberOpen(false)}
          onAdded={() => {
            setAddMemberOpen(false)
            members.refetch()
          }}
        />
      )}

      {editingMemberRow && (
        <EditMemberDialog
          libraryEntryId={id}
          personId={editingMemberRow.personId}
          personName={editingMemberRow.name}
          role={editingMemberRow.role}
          startDate={editingMemberRow.startDate}
          endDate={editingMemberRow.endDate}
          onClose={() => setEditingMemberRow(undefined)}
          onSaved={() => {
            setEditingMemberRow(undefined)
            members.refetch()
          }}
        />
      )}

      {removingMemberRow && (
        <RemoveMemberDialog
          libraryEntryId={id}
          personId={removingMemberRow.personId}
          personName={removingMemberRow.name}
          role={removingMemberRow.role}
          onClose={() => setRemovingMemberRow(undefined)}
          onRemoved={() => {
            setRemovingMemberRow(undefined)
            members.refetch()
          }}
        />
      )}
    </div>
  )
}
