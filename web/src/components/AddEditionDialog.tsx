import { useMutation, useQuery } from '@connectrpc/connect-query'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { Disc3 } from 'lucide-react'
import { useState } from 'react'
import { listReleasesForReleaseGroup } from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import type { MusicBrainzRelease } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { createMusicRelease } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import { ReleaseStatus, type Release } from '../gen/purser/music/v1/release_pb'
import { EmptyState } from './EmptyState'
import { Modal } from './Modal'

// releaseSummary joins a MusicBrainzRelease's secondary details with a
// separator, skipping any that are empty — same convention
// AddAlbumDialog's releaseGroupSummary/AddArtistDialog's artistSummary use.
function releaseSummary(release: MusicBrainzRelease): string {
  return [release.country, release.date, release.format, release.label].filter(Boolean).join(' · ')
}

export interface AddEditionDialogProps {
  // groupId/libraryEntryId are the Album's own kernel identity —
  // every Release created here is attached to both, same denormalized
  // LibraryEntryId shape music.Release itself carries (ADR 0021).
  groupId: string
  libraryEntryId: string
  // releaseGroupMbid is the Album's own MusicBrainz release-group
  // identity — required to browse its editions via
  // ListReleasesForReleaseGroup. The caller is responsible for not
  // rendering this dialog until it's known (see AlbumDetail's own
  // gating, mirroring ArtistDetail's provider.mbid gate for Add Album).
  releaseGroupMbid: string
  onClose: () => void
  onAdded: (release: Release) => void
}

// AddEditionDialog is #675's MusicBrainz-search Add Edition path: browse
// this album's known editions via ListReleasesForReleaseGroup, then a
// single CreateMusicRelease call once one is picked. Unlike Add Album
// (#669) this is *not* ADR 0026's 3-step get-or-create dance — a
// non-empty MBID on a music.Release already runs the get-or-create branch
// server-side inside MusicReleaseRepository.Create (ADR 0021's own
// MBID-on-the-row exception to the shared ExternalID pattern), so there is
// no client-side composition to write. Presentation only, same
// props-in/callbacks-out split AddAlbumDialog established.
//
// Every field MusicBrainzRelease actually returns is mapped
// (title/country/date/label/catalog_number/barcode/format/medium_count/
// track_count) — status is always Purser's own ReleaseStatusStub (never
// derived from MusicBrainz's own release status vocabulary), is_default
// is always false (see #677), and date, a plain string on the wire, is
// parsed via the same timestampFromDate(new Date(...)) convention
// PersonDialog's birth/death date fields use.
export function AddEditionDialog({ groupId, libraryEntryId, releaseGroupMbid, onClose, onAdded }: AddEditionDialogProps) {
  const [addingMbid, setAddingMbid] = useState<string | null>(null)
  const [addError, setAddError] = useState<string | null>(null)
  const createMutation = useMutation(createMusicRelease)

  const { data, isPending, isError, error } = useQuery(
    listReleasesForReleaseGroup,
    { releaseGroupMbid },
    { enabled: releaseGroupMbid !== '' },
  )
  const releases = data?.releases ?? []

  function handleSelect(candidate: MusicBrainzRelease) {
    setAddError(null)
    setAddingMbid(candidate.mbid)
    createMutation.mutate(
      {
        musicRelease: {
          groupId,
          libraryEntryId,
          title: candidate.title,
          country: candidate.country,
          date: candidate.date ? timestampFromDate(new Date(candidate.date)) : undefined,
          label: candidate.label,
          catalogNumber: candidate.catalogNumber,
          barcode: candidate.barcode,
          format: candidate.format,
          mediumCount: candidate.mediumCount,
          trackCount: candidate.trackCount,
          status: ReleaseStatus.STUB,
          isDefault: false,
          mbid: candidate.mbid,
        },
      },
      {
        onSuccess: response => {
          setAddingMbid(null)
          if (response.musicRelease) onAdded(response.musicRelease)
        },
        onError: err => {
          setAddingMbid(null)
          setAddError(err instanceof Error ? err.message : 'Could not add this edition.')
        },
      },
    )
  }

  return (
    <Modal title="Add Edition" onClose={onClose}>
      <div className="flex flex-col gap-4">
        {isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't load this album's editions ({error.message}).
          </p>
        )}

        {isPending && <p className="text-body text-text-secondary">Loading editions…</p>}

        {!isPending && !isError && releases.length === 0 && (
          <EmptyState icon={Disc3} title="No editions found" description="MusicBrainz has no releases for this album." />
        )}

        {releases.length > 0 && (
          <ul className="flex max-h-96 flex-col gap-1 overflow-y-auto">
            {releases.map(release => (
              <li key={release.mbid}>
                <button
                  type="button"
                  onClick={() => handleSelect(release)}
                  disabled={addingMbid !== null}
                  className="flex w-full flex-col items-start gap-0.5 rounded-lg px-3 py-2 text-left hover:bg-surface disabled:opacity-50"
                >
                  <span className="text-body font-medium text-text">
                    {release.title}
                    {release.disambiguation !== '' && (
                      <span className="ml-2 text-label text-text-secondary">{release.disambiguation}</span>
                    )}
                  </span>
                  <span className="text-label text-text-secondary">
                    {releaseSummary(release)}
                    {addingMbid === release.mbid && ' · Adding…'}
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
