import { useQuery } from '@connectrpc/connect-query'
import { listMusicReleaseTracks } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'
import { ItemStatus } from '../gen/purser/domain/v1/common_pb'
import type { ItemStatus as PlainItemStatus } from '../types'

// TRACKS_PAGE_SIZE — same "large enough a real per-parent list never
// needs a second page" precedent as useDiscography's RELEASES_PAGE_SIZE:
// this fan-out is bounded to one edition's tracks.
const TRACKS_PAGE_SIZE = 200

// itemStatusFromProto maps the wire ItemStatus enum to the plain string
// union ItemStatusBadge (#659) takes — the same "no UNSPECIFIED case, it's
// a required validated oneof server-side" precedent
// useDiscography.statusFromProto establishes for Release. #676 is this
// mapping's first real caller — ItemStatusBadge existed but was never
// wired to a real Item before.
export function itemStatusFromProto(status: ItemStatus): PlainItemStatus | undefined {
  switch (status) {
    case ItemStatus.WANTED:
      return 'wanted'
    case ItemStatus.GRABBED:
      return 'grabbed'
    case ItemStatus.DOWNLOADING:
      return 'downloading'
    case ItemStatus.IMPORTED:
      return 'imported'
    case ItemStatus.MISSING:
      return 'missing'
    case ItemStatus.SKIPPED:
      return 'skipped'
    default:
      return undefined
  }
}

// useReleaseTracks wraps MusicReleaseService.ListMusicReleaseTracks —
// the live consumer Album Detail's Editions strip (#674) drives by
// selecting an edition, proving the selection is actually wired to a
// query rather than a visual-only tab (#674's own acceptance criterion).
// #676 builds the real per-track rows/status badges/play icon/Add-track UI
// on top of this hook; refetch is exposed so track-mutating actions
// (Add track, Populate from MusicBrainz, Delete) can refresh the list
// without a full page reload.
export function useReleaseTracks(releaseId: string) {
  const query = useQuery(
    listMusicReleaseTracks,
    { releaseId, pageSize: TRACKS_PAGE_SIZE, pageToken: '' },
    { enabled: !!releaseId },
  )

  return {
    tracks: query.data?.tracks ?? [],
    isPending: query.isPending,
    refetch: query.refetch,
  }
}
