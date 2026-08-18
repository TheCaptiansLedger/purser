import type { JsonObject } from '@bufbuild/protobuf'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { useMemo } from 'react'
import { updateLibraryEntry } from '../gen/purser/domain/v1/library_entry-LibraryEntryService_connectquery'
import type { LibraryEntry } from '../gen/purser/domain/v1/library_entry_pb'
import { getArtist } from '../gen/purser/music/v1/musicbrainz_search-MusicBrainzService_connectquery'
import type { GetMusicBrainzArtistResponse } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { aliasesField, stringField } from '../lib/metadataFields'

// ArtistMetadataDiffField is one field this action can refresh. `path` is
// the top-level LibraryEntry field the change ultimately lands under —
// `metadata` for every Metadata sub-key, since UpdateLibraryEntry's
// field-mask only masks `metadata` atomically (applyLibraryEntryFieldMask
// has no per-sub-key path), never a bare "name"/"sort_name" for those.
// `metadataKey` is set only when `path === 'metadata'`.
export interface ArtistMetadataDiffField {
  path: 'name' | 'sort_name' | 'metadata'
  metadataKey?: string
  label: string
  current?: string | string[]
  proposed: string | string[]
}

function arraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false
  const sortedA = [...a].sort()
  const sortedB = [...b].sort()
  return sortedA.every((value, index) => value === sortedB[index])
}

// pushStringIfChanged/pushAliasesIfChanged skip a field entirely when
// MusicBrainz's own value is empty — never propose clearing existing data
// over a provider gap, same "empty strings/arrays are omitted" rule
// useAddArtist.ts's own MusicBrainz-relations mapping already documents.
function pushStringIfChanged(
  diff: ArtistMetadataDiffField[],
  path: ArtistMetadataDiffField['path'],
  label: string,
  current: string | undefined,
  proposed: string,
  metadataKey?: string,
) {
  if (proposed === '' || proposed === (current ?? '')) return
  diff.push({ path, metadataKey, label, current, proposed })
}

function pushAliasesIfChanged(diff: ArtistMetadataDiffField[], current: string[], proposed: string[]) {
  if (proposed.length === 0 || arraysEqual(current, proposed)) return
  diff.push({ path: 'metadata', metadataKey: 'aliases', label: 'Aliases', current, proposed })
}

// computeArtistMetadataDiff diffs a LibraryEntry against
// MusicBrainzService.GetArtist's response, per ADR 0021's reuse table:
// Name/SortName plus Metadata's artist_type/aliases/founded_date-or-
// born_date/dissolved_date-or-died_date/isni/official_url/wikipedia_url.
// Founded/dissolved vs born/died is chosen by artist.type === "Person",
// the same branch ArtistDetail's facts sidebar and useAddArtist's own
// mapping already use — only the currently-matching pair is compared, so
// a type change between imports isn't reconciled here.
//
// A field absent from this list entirely (no diff row) means either its
// MusicBrainz value is empty or it already matches — both read as "no
// change to propose", the caller can't tell which and doesn't need to.
export function computeArtistMetadataDiff(
  entry: LibraryEntry,
  response: GetMusicBrainzArtistResponse,
): ArtistMetadataDiffField[] {
  const artist = response.artist
  if (!artist) return []

  const diff: ArtistMetadataDiffField[] = []
  const metadata = entry.metadata

  pushStringIfChanged(diff, 'name', 'Name', entry.name, artist.name)
  pushStringIfChanged(diff, 'sort_name', 'Sort name', entry.sortName, artist.sortName)
  pushStringIfChanged(diff, 'metadata', 'Artist type', stringField(metadata, 'artist_type'), artist.type, 'artist_type')
  pushAliasesIfChanged(diff, aliasesField(metadata), artist.aliases)

  const isPerson = artist.type === 'Person'
  const beginKey = isPerson ? 'born_date' : 'founded_date'
  const endKey = isPerson ? 'died_date' : 'dissolved_date'
  pushStringIfChanged(
    diff,
    'metadata',
    isPerson ? 'Born' : 'Founded',
    stringField(metadata, beginKey),
    artist.lifeSpanBegin,
    beginKey,
  )
  pushStringIfChanged(
    diff,
    'metadata',
    isPerson ? 'Died' : 'Dissolved',
    stringField(metadata, endKey),
    artist.lifeSpanEnd,
    endKey,
  )

  pushStringIfChanged(diff, 'metadata', 'ISNI', stringField(metadata, 'isni'), response.isnis[0] ?? '', 'isni')
  pushStringIfChanged(
    diff,
    'metadata',
    'Official site',
    stringField(metadata, 'official_url'),
    response.officialUrl,
    'official_url',
  )
  pushStringIfChanged(
    diff,
    'metadata',
    'Wikipedia',
    stringField(metadata, 'wikipedia_url'),
    response.wikipediaUrl,
    'wikipedia_url',
  )

  return diff
}

export interface ArtistMetadataUpdate {
  libraryEntry: { id: string; name?: string; sortName?: string; metadata?: JsonObject }
  updateMask: { paths: string[] }
}

// buildArtistMetadataUpdate turns a diff into an UpdateLibraryEntry
// payload whose field-mask is exactly the set of top-level paths the diff
// actually touched — never a field the diff view didn't show as changed
// (#671's acceptance criterion). A changed metadata sub-key patches a copy
// of the entry's existing metadata rather than replacing it wholesale,
// since `metadata` is one atomic field-mask path.
export function buildArtistMetadataUpdate(entry: LibraryEntry, diff: ArtistMetadataDiffField[]): ArtistMetadataUpdate {
  const paths = new Set<string>()
  const libraryEntry: ArtistMetadataUpdate['libraryEntry'] = { id: entry.id }
  const metadataPatch: JsonObject = { ...(entry.metadata ?? {}) }

  for (const field of diff) {
    paths.add(field.path)
    if (field.path === 'name') libraryEntry.name = field.proposed as string
    else if (field.path === 'sort_name') libraryEntry.sortName = field.proposed as string
    else if (field.metadataKey) metadataPatch[field.metadataKey] = field.proposed
  }

  if (paths.has('metadata')) libraryEntry.metadata = metadataPatch

  return { libraryEntry, updateMask: { paths: [...paths] } }
}

// useRefreshArtistMetadata is RefreshArtistMetadataDialog's (#671) data
// hook: MusicBrainzService.GetArtist(mbid) (read-only passthrough, per
// ADR 0027), diffed against `entry` via computeArtistMetadataDiff, and an
// `apply()` that submits buildArtistMetadataUpdate's minimal-field-mask
// payload through the existing UpdateLibraryEntry RPC. The dialog is the
// only caller — kept separate from the page's own useLibraryEntry/
// useUpdateLibraryEntryMutation so it only runs while the dialog is
// mounted.
export function useRefreshArtistMetadata(entry: LibraryEntry, mbid: string) {
  const artistQuery = useQuery(getArtist, { mbid }, { enabled: mbid !== '', retry: false })
  const updateMutation = useMutation(updateLibraryEntry)

  const diff = useMemo(
    () => (artistQuery.data ? computeArtistMetadataDiff(entry, artistQuery.data) : []),
    [entry, artistQuery.data],
  )

  async function apply(): Promise<LibraryEntry | undefined> {
    const result = await updateMutation.mutateAsync(buildArtistMetadataUpdate(entry, diff))
    return result.libraryEntry
  }

  return {
    isPending: artistQuery.isPending,
    isError: artistQuery.isError,
    error: artistQuery.error,
    diff,
    apply,
    isApplying: updateMutation.isPending,
  }
}
