import { useQuery } from '@connectrpc/connect-query'
import { EntityType } from '../gen/purser/domain/v1/common_pb'
import { getExternalID } from '../gen/purser/domain/v1/external_id-ExternalIDService_connectquery'
import { lookupArtist as lookupFanartTVArtist } from '../gen/purser/music/v1/fanarttv-FanartTVService_connectquery'
import { lookupArtist as lookupTheAudioDBArtist } from '../gen/purser/music/v1/theaudiodb-TheAudioDBService_connectquery'
import type { ArtworkCandidate } from '../components/ChooseArtworkDialog'

const MBZ_SOURCE = 'mbz'

// useArtistProviderData is the Artist Detail page's (#666) read-only
// provider fan-out: ExternalIDService.GetExternalID(LIBRARY_ENTRY, id,
// "mbz") for the artist's MBID, then, only once that resolves,
// FanartTVService.LookupArtist(mbid) (Hero backdrop/logo candidates +
// poster/thumb candidates) and TheAudioDBService.LookupArtist(mbid) (bio
// panel). Per ADR 0027 this is two independent, unranked provider calls
// — nothing here merges or picks a winner between them.
//
// Every step is best-effort: no `mbz` ExternalID row, or a 404 from
// either provider, degrades to that section simply being absent —
// `retry: false` and `enabled` gating keep a miss from ever surfacing as
// a page-level error, per this issue's own graceful-partial-data
// acceptance criterion. Nothing here is persisted (0027) — it's fetched
// fresh on every visit.
export function useArtistProviderData(libraryEntryId: string) {
  const externalIdQuery = useQuery(
    getExternalID,
    { entityType: EntityType.LIBRARY_ENTRY, entityId: libraryEntryId, source: MBZ_SOURCE },
    { enabled: !!libraryEntryId, retry: false },
  )
  const mbid = externalIdQuery.data?.externalId?.value

  const fanartQuery = useQuery(lookupFanartTVArtist, { mbid: mbid ?? '' }, { enabled: !!mbid, retry: false })
  const audiodbQuery = useQuery(lookupTheAudioDBArtist, { mbid: mbid ?? '' }, { enabled: !!mbid, retry: false })

  const artistBackground = fanartQuery.data?.artist?.artistBackground ?? []
  const hdMusicLogo = fanartQuery.data?.artist?.hdMusicLogo ?? []
  const artistThumb = fanartQuery.data?.artist?.artistThumb ?? []
  const backdropUrl = artistBackground[0]?.url

  // artist_background/hd_music_logo are presented as candidates for the
  // Hero backdrop; artist_thumb (fanart.tv's poster-shaped image) is
  // presented as candidates for the poster. Each field feeds exactly one
  // slot, both offered rather than the server silently preferring one,
  // same no-ranking rule as any other provider (0027).
  const backdropCandidates: ArtworkCandidate[] = [
    ...artistBackground.map(image => ({ url: image.url, source: 'fanart.tv', label: 'Background' })),
    ...hdMusicLogo.map(image => ({ url: image.url, source: 'fanart.tv', label: 'Logo' })),
  ]
  const posterCandidates: ArtworkCandidate[] = artistThumb.map(image => ({
    url: image.url,
    source: 'fanart.tv',
    label: 'Thumb',
  }))

  const biography = audiodbQuery.data?.artist?.biography
  const bio = biography && biography !== '' ? biography : undefined

  return { mbid, backdropUrl, backdropCandidates, posterCandidates, bio }
}
