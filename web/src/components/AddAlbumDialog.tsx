import { useQuery } from '@connectrpc/connect-query'
import { Disc3 } from 'lucide-react'
import { useState } from 'react'
import type { Group } from '../gen/purser/domain/v1/group_pb'
import { listReleaseGroupsForArtist } from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import type { MusicBrainzReleaseGroup } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { useAddAlbum } from '../hooks/useAddAlbum'
import { EmptyState } from './EmptyState'
import { Modal } from './Modal'

// releaseGroupSummary joins the row's secondary details (type, secondary
// types, first release date) with a separator, skipping any that are
// empty — same convention as AddArtistDialog's artistSummary.
function releaseGroupSummary(releaseGroup: MusicBrainzReleaseGroup): string {
  return [releaseGroup.primaryType, releaseGroup.secondaryTypes.join(', '), releaseGroup.firstReleaseDate]
    .filter(Boolean)
    .join(' · ')
}

export interface AddAlbumDialogProps {
  // artistId is the current artist's LibraryEntry.ID — every Group
  // created here is attached to it (#669's scope: Add Album is reached
  // from Artist Detail's Discography tab, always for the artist already
  // on screen).
  artistId: string
  // artistMbid is the artist's MusicBrainz identity — required to browse
  // their discography via ListReleaseGroupsForArtist. The caller is
  // responsible for not rendering this dialog until it's known (see
  // ArtistDetail's own gating).
  artistMbid: string
  onClose: () => void
  // onAdded fires once the get-or-create composition resolves to a final
  // Group (whether newly created or already existing) — the caller
  // decides what "done" means. Unlike AddArtistDialog, this never
  // navigates: there is no Album Detail page yet (#673), so the caller
  // just refreshes its own album grid.
  onAdded: (group: Group) => void
}

// AddAlbumDialog is #669's Add Album flow: browse the current artist's
// MusicBrainz discography via ListReleaseGroupsForArtist, then
// useAddAlbum's ADR 0026 get-or-create composition once a result is
// picked. This dialog is presentation only — the composition logic
// lives entirely in useAddAlbum so it can be unit tested independent of
// any rendering, same split AddArtistDialog/useAddArtist established.
export function AddAlbumDialog({ artistId, artistMbid, onClose, onAdded }: AddAlbumDialogProps) {
  const { addAlbum } = useAddAlbum()
  const [addingMbid, setAddingMbid] = useState<string | null>(null)
  const [addError, setAddError] = useState<string | null>(null)

  const { data, isPending, isError, error } = useQuery(
    listReleaseGroupsForArtist,
    { artistMbid },
    { enabled: artistMbid !== '' },
  )
  const releaseGroups = data?.releaseGroups ?? []

  async function handleSelect(candidate: MusicBrainzReleaseGroup) {
    setAddError(null)
    setAddingMbid(candidate.mbid)
    try {
      const group = await addAlbum(candidate, artistId)
      onAdded(group)
    } catch (err) {
      setAddError(err instanceof Error ? err.message : 'Could not add this album.')
    } finally {
      setAddingMbid(null)
    }
  }

  return (
    <Modal title="Add Album" onClose={onClose}>
      <div className="flex flex-col gap-4">
        {isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't load this artist's discography ({error.message}).
          </p>
        )}

        {isPending && <p className="text-body text-text-secondary">Loading discography…</p>}

        {!isPending && !isError && releaseGroups.length === 0 && (
          <EmptyState icon={Disc3} title="No albums found" description="MusicBrainz has no release groups for this artist." />
        )}

        {releaseGroups.length > 0 && (
          <ul className="flex max-h-96 flex-col gap-1 overflow-y-auto">
            {releaseGroups.map(releaseGroup => (
              <li key={releaseGroup.mbid}>
                <button
                  type="button"
                  onClick={() => handleSelect(releaseGroup)}
                  disabled={addingMbid !== null}
                  className="flex w-full flex-col items-start gap-0.5 rounded-lg px-3 py-2 text-left hover:bg-surface disabled:opacity-50"
                >
                  <span className="text-body font-medium text-text">
                    {releaseGroup.title}
                    {releaseGroup.disambiguation !== '' && (
                      <span className="ml-2 text-label text-text-secondary">{releaseGroup.disambiguation}</span>
                    )}
                  </span>
                  <span className="text-label text-text-secondary">
                    {releaseGroupSummary(releaseGroup)}
                    {addingMbid === releaseGroup.mbid && ' · Adding…'}
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
