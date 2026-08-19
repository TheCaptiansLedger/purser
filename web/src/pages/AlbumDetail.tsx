import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Camera, Disc3, Images, Maximize2, Plus } from 'lucide-react'
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { AddEditionDialog } from '../components/AddEditionDialog'
import { ChooseArtworkDialog } from '../components/ChooseArtworkDialog'
import { DropdownMenu } from '../components/DropdownMenu'
import { EditionsStrip } from '../components/EditionsStrip'
import { EmptyState } from '../components/EmptyState'
import { Hero } from '../components/Hero'
import { ImageGallery } from '../components/ImageGallery'
import { ImageLightbox } from '../components/ImageLightbox'
import { ManualEditionDialog } from '../components/ManualEditionDialog'
import { Tracklist } from '../components/Tracklist'
import { EntityType } from '../gen/purser/domain/v1/common_pb'
import { getExternalID } from '../gen/purser/domain/v1/external_id-ExternalIDService_connectquery'
import { getGroup } from '../gen/purser/domain/v1/group-GroupService_connectquery'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
import {
  listMusicReleases,
  updateMusicRelease,
} from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import type { Release } from '../gen/purser/music/v1/release_pb'
import { useGroupTags } from '../hooks/useGroupTags'
import { useReleaseTracks } from '../hooks/useReleaseTracks'

const MBZ_SOURCE = 'mbz'

const RELEASES_PAGE_SIZE = 50

// AlbumDetail — the Album (Release Group) Detail page shell (#673):
// GroupService.GetGroup, a Hero (#658) with no backdrop (no artwork
// source for one yet — unlike Artist Detail's fanart.tv backdrop) and
// facts=[year, track count]. The metadata editor (#681) is a separate,
// later issue — this page reads the default edition
// (MusicReleaseService.ListMusicReleases, same `isDefault ?? [0]`
// fallback useDiscography already established) to know which edition owns
// the cover art and to report its track count in the Hero facts,
// independent of whichever edition is selected in the strip below.
//
// Add Edition (#675) is the same two-source DropdownMenu shape #669
// established for Add Album, sitting above the strip: "Search
// MusicBrainz" opens AddEditionDialog (disabled until the release
// group's own `mbz` ExternalID is known — ExternalIDService
// .GetExternalID(GROUP, id, "mbz"), same ArtistDetail/provider.mbid
// pattern), "Add Manually" opens ManualEditionDialog. Both dialogs call
// MusicReleaseService.CreateMusicRelease directly — no get-or-create
// composition needed client-side, since a non-empty MBID already runs
// the server-side get-or-create branch (MusicReleaseRepository.Create,
// ADR 0021's own MBID-on-the-row exception to the shared ExternalID
// pattern). A zero-editions album renders an EmptyState with the same
// dropdown rather than hiding it, so the first edition can be added.
//
// Editions strip (#674): selecting a card updates selectedReleaseId
// (optimistic local mirror pattern, same `x ?? entry.x` shape
// ArtistDetail's `monitored` state uses, defaulting to the default
// edition), which drives useReleaseTracks, rendered via Tracklist (#676) —
// per-track rows/status badges/play icon plus the Add track (manual +
// Populate from MusicBrainz)/Delete write path #721/#720 added. Per-edition
// Monitored toggles call UpdateMusicRelease
// (field-mask [monitored]) with the same optimistic-write/rollback shape
// as ArtistDetail's handleMonitorToggle, keyed per release since several
// cards can be mid-toggle at once.
//
// Set default edition (#677): two sequential UpdateMusicRelease calls
// (field-mask [is_default]) — unset the current default, then set the
// chosen one — since MusicRelease rows have no cross-row atomicity (same
// reasoning #680's bulk-monitor story accepts) and the server does no
// auto-unset of its own (confirmed against music_release_convert.go).
// Only the second call's failure gets the specific "No default edition is
// currently set" copy the issue calls for, because that's the only case
// where it's actually true — the first call already went through, so the
// old default really is gone. A first-call failure leaves the old default
// untouched, so it gets a distinct message rather than falsely claiming
// no default exists. Both cases refetch is skipped on failure (nothing
// changed to reflect) and offer the same Retry action, which just
// re-invokes handleSetDefault for the same release.
//
// Cover art attaches to the default edition, not the Group itself, per
// ADR 0021's "Cover art: Image... owned by the release" — ownerType=
// "music_release" (same "poster" imageType convention library_entry/
// group covers already use), through #656's ChooseArtworkDialog/
// ImageGallery and viewed via #655's ImageLightbox. Upload-only: no
// provider wires a release-cover candidate yet, unlike Artist Detail's
// fanart.tv poster/backdrop. A Group with zero editions has nothing to
// own the image, so the cover art controls are hidden rather than
// pointed at a release that doesn't exist.
//
// Genre/mood chips — TagAssignmentService.ListTagAssignments(entity_type
// =GROUP) + TagService.GetTag per assignment (see useGroupTags), scope=
// metadata per ADR 0021. An album with zero tag assignments renders no
// chip row at all, not an empty placeholder chip (#673's own acceptance
// criterion).
export function AlbumDetail() {
  const { id = '' } = useParams<{ id: string }>()
  const groupQuery = useQuery(getGroup, { id }, { enabled: !!id })
  const releasesQuery = useQuery(
    listMusicReleases,
    { pageSize: RELEASES_PAGE_SIZE, pageToken: '', groupId: id, libraryEntryId: '' },
    { enabled: !!id },
  )
  const tagsQuery = useGroupTags(id)
  const updateReleaseMutation = useMutation(updateMusicRelease)
  const releaseGroupMbidQuery = useQuery(
    getExternalID,
    { entityType: EntityType.GROUP, entityId: id, source: MBZ_SOURCE },
    { enabled: !!id, retry: false },
  )
  const releaseGroupMbid = releaseGroupMbidQuery.data?.externalId?.value

  const [coverLightboxOpen, setCoverLightboxOpen] = useState(false)
  const [coverDialogOpen, setCoverDialogOpen] = useState(false)
  const [coverGalleryOpen, setCoverGalleryOpen] = useState(false)
  const [addEditionOpen, setAddEditionOpen] = useState(false)
  const [manualEditionOpen, setManualEditionOpen] = useState(false)
  // Optimistic local mirrors — same `x ?? entry.x` pattern ArtistDetail's
  // `monitored` state uses. monitoredOverrides is keyed per release id
  // since the strip can have several cards mid-toggle at once, unlike
  // ArtistDetail's single LibraryEntry-level toggle.
  const [selectedReleaseId, setSelectedReleaseId] = useState<string | undefined>(undefined)
  const [monitoredOverrides, setMonitoredOverrides] = useState<Record<string, boolean>>({})
  const [pendingReleaseId, setPendingReleaseId] = useState<string | undefined>(undefined)
  const [settingDefaultReleaseId, setSettingDefaultReleaseId] = useState<string | undefined>(undefined)
  const [defaultAssignError, setDefaultAssignError] = useState<{ releaseId: string; noDefault: boolean } | null>(
    null,
  )

  const releases = releasesQuery.data?.musicReleases ?? []
  const defaultRelease = releases.find(release => release.isDefault) ?? releases[0]
  const activeReleaseId = selectedReleaseId ?? defaultRelease?.id ?? ''
  const activeRelease = releases.find(release => release.id === activeReleaseId)

  const coverQuery = useQuery(
    getSelectedImage,
    { ownerType: 'music_release', ownerId: defaultRelease?.id ?? '', imageType: 'poster' },
    { enabled: !!defaultRelease?.id, retry: false },
  )

  // The Editions strip's (#674) live consumer — proves selecting an
  // edition actually drives a query rather than being a visual-only tab.
  // Tracklist (#676) is the real UI built on top of this same hook.
  const tracksQuery = useReleaseTracks(activeReleaseId)

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
  if (groupQuery.isPending) {
    return null
  }

  if (groupQuery.isError) {
    return (
      <p className="mx-6 mt-10 text-body text-status-failure" role="alert">
        Couldn't load this album ({groupQuery.error.message}).
      </p>
    )
  }

  const group = groupQuery.data.group
  if (!group) {
    return (
      <p className="mx-6 mt-10 text-body text-status-failure" role="alert">
        Album not found.
      </p>
    )
  }

  const coverImageId = coverQuery.data?.image?.id
  const coverSrc = coverImageId ? `/media/images/${coverImageId}` : undefined

  const facts = [
    group.year > 0 ? String(group.year) : undefined,
    defaultRelease && defaultRelease.trackCount > 0
      ? `${defaultRelease.trackCount} track${defaultRelease.trackCount === 1 ? '' : 's'}`
      : undefined,
  ].filter((fact): fact is string => !!fact)

  function handleToggleMonitored(release: Release, next: boolean) {
    const previous = monitoredOverrides[release.id] ?? release.monitored
    setMonitoredOverrides(overrides => ({ ...overrides, [release.id]: next }))
    setPendingReleaseId(release.id)
    updateReleaseMutation.mutate(
      { musicRelease: { id: release.id, monitored: next }, updateMask: { paths: ['monitored'] } },
      {
        onSuccess: response =>
          setMonitoredOverrides(overrides => ({
            ...overrides,
            [release.id]: response.musicRelease?.monitored ?? next,
          })),
        onError: () => setMonitoredOverrides(overrides => ({ ...overrides, [release.id]: previous })),
        onSettled: () => setPendingReleaseId(undefined),
      },
    )
  }

  async function handleSetDefault(release: Release) {
    setDefaultAssignError(null)
    setSettingDefaultReleaseId(release.id)

    const currentDefault = releases.find(r => r.isDefault)
    if (currentDefault && currentDefault.id !== release.id) {
      try {
        await updateReleaseMutation.mutateAsync({
          musicRelease: { id: currentDefault.id, isDefault: false },
          updateMask: { paths: ['is_default'] },
        })
      } catch {
        setSettingDefaultReleaseId(undefined)
        setDefaultAssignError({ releaseId: release.id, noDefault: false })
        return
      }
    }

    try {
      await updateReleaseMutation.mutateAsync({
        musicRelease: { id: release.id, isDefault: true },
        updateMask: { paths: ['is_default'] },
      })
      releasesQuery.refetch()
    } catch {
      setDefaultAssignError({ releaseId: release.id, noDefault: true })
    } finally {
      setSettingDefaultReleaseId(undefined)
    }
  }

  return (
    <div className="px-6 py-10 md:px-8">
      <Hero title={group.title} facts={facts} />

      <div className="mt-6 flex flex-col gap-6 sm:flex-row sm:items-start">
        <div className="flex flex-col items-center gap-2 sm:w-48 shrink-0">
          {coverSrc ? (
            <button
              type="button"
              onClick={() => setCoverLightboxOpen(true)}
              aria-label={`View ${group.title}'s cover art`}
              className="aspect-square w-full overflow-hidden rounded-xl border border-border"
            >
              <img src={coverSrc} alt={group.title} className="h-full w-full object-cover" />
            </button>
          ) : (
            <div
              aria-hidden="true"
              className="flex aspect-square w-full items-center justify-center rounded-xl border border-border bg-surface-raised text-text-secondary"
            >
              <Disc3 size={40} />
            </div>
          )}

          {defaultRelease && (
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setCoverDialogOpen(true)}
                className="flex h-8 items-center gap-2 rounded-lg px-3 text-label font-medium text-text-secondary hover:bg-surface-raised hover:text-text"
              >
                <Camera size={14} />
                {coverSrc ? 'Change cover' : 'Add cover'}
              </button>

              <button
                type="button"
                onClick={() => setCoverGalleryOpen(true)}
                aria-label="Manage cover art"
                title="Manage cover art"
                className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text"
              >
                <Images size={14} />
              </button>

              {coverSrc && (
                <button
                  type="button"
                  onClick={() => setCoverLightboxOpen(true)}
                  aria-label="View cover art full-screen"
                  title="View cover art full-screen"
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text"
                >
                  <Maximize2 size={14} />
                </button>
              )}
            </div>
          )}
        </div>

        {tagsQuery.tags.length > 0 && (
          <div className="flex flex-1 flex-wrap items-start gap-2">
            {tagsQuery.tags.map(tag => (
              <span
                key={tag.id}
                className="rounded-md border border-border bg-surface-raised px-2 py-1 text-label text-text-secondary"
              >
                {tag.value}
              </span>
            ))}
          </div>
        )}
      </div>

      <div className="mt-8">
        <div className="mb-4 flex justify-end">
          <DropdownMenu
            label="Add Edition"
            trigger={
              <>
                <Plus size={16} aria-hidden="true" />
                Add Edition
              </>
            }
            triggerClassName="flex h-9 items-center gap-1.5 rounded-lg bg-accent-system px-4 text-body font-medium text-bg hover:opacity-90"
            items={[
              {
                label: 'Search MusicBrainz',
                onSelect: () => setAddEditionOpen(true),
                disabled: !releaseGroupMbid,
              },
              { label: 'Add Manually', onSelect: () => setManualEditionOpen(true) },
            ]}
          />
        </div>

        {releases.length === 0 && (
          <EmptyState icon={Disc3} title="No editions yet" description="Editions added to this album will show up here." />
        )}

        {defaultAssignError && (
          <div
            role="alert"
            className="mb-4 flex items-center justify-between gap-3 rounded-lg border border-status-failure/40 bg-surface-raised px-4 py-3 text-body text-status-failure"
          >
            <span>
              {defaultAssignError.noDefault
                ? 'No default edition is currently set.'
                : "Couldn't set that edition as default."}
            </span>
            <button
              type="button"
              onClick={() => {
                const target = releases.find(r => r.id === defaultAssignError.releaseId)
                if (target) handleSetDefault(target)
              }}
              className="shrink-0 rounded-lg px-3 py-1 text-label font-medium text-status-failure hover:bg-status-failure/10"
            >
              Retry
            </button>
          </div>
        )}

        {releases.length > 0 && (
          <>
            <EditionsStrip
              releases={releases}
              selectedId={activeReleaseId}
              onSelect={setSelectedReleaseId}
              onToggleMonitored={handleToggleMonitored}
              monitoredOverrides={monitoredOverrides}
              pendingReleaseId={pendingReleaseId}
              onSetDefault={handleSetDefault}
              settingDefaultReleaseId={settingDefaultReleaseId}
            />

            <Tracklist
              releaseId={activeReleaseId}
              mbid={activeRelease?.mbid ?? ''}
              tracks={tracksQuery.tracks}
              isPending={tracksQuery.isPending}
              refetch={() => void tracksQuery.refetch()}
            />
          </>
        )}
      </div>

      {coverLightboxOpen && coverSrc && (
        <ImageLightbox src={coverSrc} alt={group.title} onClose={() => setCoverLightboxOpen(false)} />
      )}

      {coverDialogOpen && defaultRelease && (
        <ChooseArtworkDialog
          title={`${coverSrc ? 'Change' : 'Add'} cover`}
          ownerType="music_release"
          ownerId={defaultRelease.id}
          imageType="poster"
          onClose={() => setCoverDialogOpen(false)}
          onAttached={() => void coverQuery.refetch()}
        />
      )}

      {coverGalleryOpen && defaultRelease && (
        <ImageGallery
          title="Manage cover art"
          ownerType="music_release"
          ownerId={defaultRelease.id}
          imageType="poster"
          onClose={() => setCoverGalleryOpen(false)}
          onChange={() => void coverQuery.refetch()}
        />
      )}

      {addEditionOpen && releaseGroupMbid && (
        <AddEditionDialog
          groupId={id}
          libraryEntryId={group.libraryEntryId}
          releaseGroupMbid={releaseGroupMbid}
          onClose={() => setAddEditionOpen(false)}
          onAdded={release => {
            setAddEditionOpen(false)
            setSelectedReleaseId(release.id)
            releasesQuery.refetch()
          }}
        />
      )}

      {manualEditionOpen && (
        <ManualEditionDialog
          groupId={id}
          libraryEntryId={group.libraryEntryId}
          onClose={() => setManualEditionOpen(false)}
          onAdded={release => {
            setManualEditionOpen(false)
            setSelectedReleaseId(release.id)
            releasesQuery.refetch()
          }}
        />
      )}
    </div>
  )
}
