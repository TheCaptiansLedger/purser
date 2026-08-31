import { useQuery } from '@connectrpc/connect-query'
import type { StashDBPerformer } from '../gen/purser/afterdark/v1/stashdb_pb'
import {
  lookupPerformer as lookupStashDBPerformer,
  searchPerformers as searchStashDBPerformers,
} from '../gen/purser/afterdark/v1/stashdb-StashDBService_connectquery'
import type { TPDBPerformer } from '../gen/purser/afterdark/v1/theporndb_pb'
import {
  lookupPerformer as lookupThePornDBPerformer,
  searchPerformers as searchThePornDBPerformers,
} from '../gen/purser/afterdark/v1/theporndb-ThePornDBService_connectquery'
import { EntityType } from '../gen/purser/domain/v1/common_pb'
import { getExternalID } from '../gen/purser/domain/v1/external_id-ExternalIDService_connectquery'
import { lookupArtist as lookupFanartTVArtist } from '../gen/purser/music/v1/fanarttv-FanartTVService_connectquery'
import { lookupArtist as lookupTheAudioDBArtist } from '../gen/purser/music/v1/theaudiodb-TheAudioDBService_connectquery'
import {
  getArtist as getMusicBrainzArtist,
  searchArtists as searchMusicBrainzArtists,
} from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import { lookupImage as lookupWikidataImage } from '../gen/purser/music/v1/wikidata-WikidataService_connectquery'
import type { ArtworkCandidate } from '../components/ChooseArtworkDialog'

const MBZ_SOURCE = 'mbz'
const STASHDB_SOURCE = 'stashdb'
const TPDB_SOURCE = 'tpdb'

function normalizeName(value: string): string {
  return value.trim().toLowerCase()
}

// matchesPersonName reports whether candidateName or one of candidateAliases
// is, case-insensitively, exactly personName — the filter every
// search-by-name fallback below runs its raw results through. A provider's
// free-text search is deliberately fuzzy (StashDB's own SearchPerformers
// returns near-miss/substring matches, confirmed live: searching "Alex
// Coal" also returns performers named plain "Alex", "Coal Daniels", ...),
// which is fine for a human browsing search results but wrong for this
// picker — showing a bare-"Alex" or unrelated performer's photos on a
// specific named Person's page is a correctness bug, not a style choice.
// Restricting to an exact name/alias match keeps every provider still
// searchable (per this issue's own instruction — no provider is
// off-limits by content-type or domain) while refusing to surface a
// same-provider result that isn't actually this same-named person.
function matchesPersonName(candidateName: string, candidateAliases: string[], personName: string): boolean {
  const target = normalizeName(personName)
  if (target === '') return false
  if (normalizeName(candidateName) === target) return true
  return candidateAliases.some(alias => normalizeName(alias) === target)
}

// usePersonProviderData is #703's Person-photo provider fan-out. Per
// provider, independently: if the Person already carries that provider's
// own ExternalID (the exact, authoritative case — set by a real
// MusicBrainz-relations import or AfterDark scene import), look that ID up
// directly; otherwise, if the provider supports free-text search, search
// by the Person's own name instead. A Person added by typing a name (the
// common case — "Add Manually", or an AfterDark performer with no scan
// history yet) still gets real candidates this way, not just the rare
// already-linked case.
//
//   - StashDB/ThePornDB both expose SearchPerformers — searched directly
//     by name when there's no "stashdb"/"tpdb" ExternalID.
//   - fanart.tv/TheAudioDB have no name-search capability at all, only an
//     MBID lookup — MusicBrainzService.SearchArtists(name) stands in for
//     them: its own top (most relevant) match's mbid feeds the same
//     fanart.tv/TheAudioDB/Wikidata chain the "mbz" ExternalID case uses,
//     when there's no "mbz" ExternalID row.
//
// Every provider's search results are filtered through matchesPersonName
// before use — a free-text search is deliberately fuzzy (confirmed live:
// StashDB's own SearchPerformers("Alex Coal") also returns performers
// named plain "Alex" and unrelated others), and this picker only wants
// this specific named person's photos, not a browsable results list. No
// provider is excluded by domain/content-type (every enabled provider is
// always searched) — the fix is precision within each provider's own
// results, never which providers get asked.
//
// Every step is best-effort: a missing ExternalID row plus a search miss,
// or an outright provider error, degrades to that source simply being
// absent from photoCandidates — `retry: false` and `enabled` gating keep
// any of this from ever surfacing as a page-level error. Nothing here is
// persisted (ADR 0027) — it's fetched fresh on every visit, and multiple
// exact-name-match hits (a name shared by more than one real person) are
// all shown, unranked, exactly like every other multi-candidate provider
// in this codebase — a human picks the right one by looking at the photo
// itself.
export function usePersonProviderData(personId: string, personName: string) {
  const hasName = personName.trim() !== ''

  // ---- Music-origin: fanart.tv / TheAudioDB / Wikidata, via mbid -------
  const mbzExternalIdQuery = useQuery(
    getExternalID,
    { entityType: EntityType.PERSON, entityId: personId, source: MBZ_SOURCE },
    { enabled: !!personId, retry: false },
  )
  const mbid = mbzExternalIdQuery.data?.externalId?.value

  const musicBrainzSearchQuery = useQuery(
    searchMusicBrainzArtists,
    { query: personName },
    // Waits for mbzExternalIdQuery to settle first — otherwise mbid reads
    // as undefined for one tick on every mount (query still pending) and
    // this would fire a wasted search even for an already-linked Person.
    { enabled: !mbzExternalIdQuery.isPending && !mbid && hasName, retry: false },
  )
  // MusicBrainz's own top (most relevant) match among the ones that
  // actually share this Person's name/alias stands in for an mbid we
  // don't otherwise have — never re-sorted, just the first exact-name
  // match in whatever order MusicBrainz itself returned.
  const matchedMusicBrainzArtists = (musicBrainzSearchQuery.data?.artists ?? []).filter(artist =>
    matchesPersonName(artist.name, artist.aliases, personName),
  )
  const searchedMbid = matchedMusicBrainzArtists[0]?.mbid
  const effectiveMbid = mbid ?? searchedMbid

  const fanartQuery = useQuery(lookupFanartTVArtist, { mbid: effectiveMbid ?? '' }, { enabled: !!effectiveMbid, retry: false })
  const audiodbQuery = useQuery(lookupTheAudioDBArtist, { mbid: effectiveMbid ?? '' }, { enabled: !!effectiveMbid, retry: false })
  const mbzArtistQuery = useQuery(getMusicBrainzArtist, { mbid: effectiveMbid ?? '' }, { enabled: !!effectiveMbid, retry: false })

  const wikidataUrl = mbzArtistQuery.data?.wikidataUrl
  const wikidataQuery = useQuery(lookupWikidataImage, { url: wikidataUrl ?? '' }, { enabled: !!wikidataUrl, retry: false })

  // ---- AfterDark-origin: StashDB ----------------------------------------
  const stashDBExternalIdQuery = useQuery(
    getExternalID,
    { entityType: EntityType.PERSON, entityId: personId, source: STASHDB_SOURCE },
    { enabled: !!personId, retry: false },
  )
  const stashDBId = stashDBExternalIdQuery.data?.externalId?.value
  const stashDBLookupQuery = useQuery(lookupStashDBPerformer, { id: stashDBId ?? '' }, { enabled: !!stashDBId, retry: false })
  const stashDBSearchQuery = useQuery(
    searchStashDBPerformers,
    { term: personName },
    // See musicBrainzSearchQuery's identical isPending guard above.
    { enabled: !stashDBExternalIdQuery.isPending && !stashDBId && hasName, retry: false },
  )
  const stashDBPerformers: StashDBPerformer[] = stashDBId
    ? stashDBLookupQuery.data?.performer
      ? [stashDBLookupQuery.data.performer]
      : []
    : (stashDBSearchQuery.data?.performers ?? []).filter(performer =>
        matchesPersonName(performer.name, performer.aliases, personName),
      )

  // ---- AfterDark-origin: ThePornDB --------------------------------------
  const tpdbExternalIdQuery = useQuery(
    getExternalID,
    { entityType: EntityType.PERSON, entityId: personId, source: TPDB_SOURCE },
    { enabled: !!personId, retry: false },
  )
  const tpdbId = tpdbExternalIdQuery.data?.externalId?.value
  const tpdbLookupQuery = useQuery(lookupThePornDBPerformer, { id: tpdbId ?? '' }, { enabled: !!tpdbId, retry: false })
  const tpdbSearchQuery = useQuery(
    searchThePornDBPerformers,
    { term: personName },
    // See musicBrainzSearchQuery's identical isPending guard above.
    { enabled: !tpdbExternalIdQuery.isPending && !tpdbId && hasName, retry: false },
  )
  const tpdbPerformers: TPDBPerformer[] = tpdbId
    ? tpdbLookupQuery.data?.performer
      ? [tpdbLookupQuery.data.performer]
      : []
    : (tpdbSearchQuery.data?.performers ?? []).filter(performer =>
        matchesPersonName(performer.name, performer.aliases, personName),
      )

  const audiodbArtist = audiodbQuery.data?.artist

  // Only thumb-shaped images (a square/portrait crop) are offered for a
  // Person photo — fanart.tv's artist_background/TheAudioDB's fanart*
  // fields are wide backdrop shots, the right shape for Artist Detail's
  // Hero but not for a Person's circular avatar, so they're deliberately
  // left out here.
  const photoCandidates: ArtworkCandidate[] = [
    ...(fanartQuery.data?.artist?.artistThumb ?? []).map(image => ({ url: image.url, source: 'fanart.tv', label: 'Thumb' })),
    ...(audiodbArtist?.thumb ? [{ url: audiodbArtist.thumb, source: 'theaudiodb', label: 'Thumb' }] : []),
    ...(audiodbArtist?.cutout ? [{ url: audiodbArtist.cutout, source: 'theaudiodb', label: 'Cutout' }] : []),
    ...(wikidataQuery.data?.images ?? []).map(image => ({ url: image.url, source: 'wikidata', label: 'Photo' })),
    ...stashDBPerformers.flatMap(performer =>
      performer.images.map(image => ({ url: image.url, source: 'stashdb', label: performer.name || 'Photo' })),
    ),
    ...tpdbPerformers.flatMap(performer => [
      ...(performer.image ? [{ url: performer.image, source: 'theporndb', label: performer.name || 'Photo' }] : []),
      ...(performer.thumbnail ? [{ url: performer.thumbnail, source: 'theporndb', label: performer.name || 'Thumbnail' }] : []),
      ...(performer.face ? [{ url: performer.face, source: 'theporndb', label: performer.name || 'Face' }] : []),
      ...performer.posters.map(poster => ({ url: poster.url, source: 'theporndb', label: performer.name || 'Poster' })),
    ]),
  ]

  return { mbid, stashDBId, tpdbId, photoCandidates }
}
