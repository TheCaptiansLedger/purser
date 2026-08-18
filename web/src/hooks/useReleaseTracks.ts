import { useQuery } from '@connectrpc/connect-query'
import { listMusicReleaseTracks } from '../gen/purser/music/v1/release-MusicReleaseService_connectquery'

// TRACKS_PAGE_SIZE — same "large enough a real per-parent list never
// needs a second page" precedent as useDiscography's RELEASES_PAGE_SIZE:
// this fan-out is bounded to one edition's tracks.
const TRACKS_PAGE_SIZE = 200

// useReleaseTracks wraps MusicReleaseService.ListMusicReleaseTracks —
// the live consumer Album Detail's Editions strip (#674) drives by
// selecting an edition, proving the selection is actually wired to a
// query rather than a visual-only tab (#674's own acceptance criterion).
// #676 replaces AlbumDetail's plain track-count read of this hook with
// the real per-track rows/status badges/play icon; this hook's shape
// (tracks + isPending) is already what that story needs, so it isn't
// re-derived when #676 lands.
export function useReleaseTracks(releaseId: string) {
  const query = useQuery(
    listMusicReleaseTracks,
    { releaseId, pageSize: TRACKS_PAGE_SIZE, pageToken: '' },
    { enabled: !!releaseId },
  )

  return {
    tracks: query.data?.tracks ?? [],
    isPending: query.isPending,
  }
}
