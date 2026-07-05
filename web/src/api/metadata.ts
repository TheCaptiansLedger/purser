import { get, post } from './client'
import type { ContentType, ExternalGroup, ExternalStudio, ExternalPerson, ExternalTrack, Group, Item, LibraryEntry, MonitorMode, Person, PersonRole } from '../types'

// ── Search ────────────────────────────────────────────────────────────────────

export function searchStudios(q: string, contentType?: ContentType, limit = 25) {
  return get<{ results: ExternalStudio[] }>('/metadata/search', { kind: 'studio', q, contentType, limit })
}

export function searchPeople(q: string, role?: PersonRole, limit = 25) {
  return get<{ results: ExternalPerson[] }>('/metadata/search', { kind: 'person', q, role, limit })
}

// ── Import entry ──────────────────────────────────────────────────────────────

export type AlbumFilterToken = 'studio' | 'live' | 'compilation' | 'ep' | 'single' | 'all'

export interface ImportEntryRequest {
  source: string
  externalId: string
  name: string
  overview?: string
  contentType: ContentType
  kind: string
  monitored: boolean
  monitorMode: MonitorMode
  parentExternalId?: string
  parentName?: string
  parentImageUrl?: string
  parentWebsiteUrl?: string
  imageUrl?: string
  websiteUrl?: string
  albumFilter?: AlbumFilterToken[]
}

// entryKindForContentType mirrors Go's ContentType.ParentEntryKind().
// Keep in sync with internal/domain/content_type.go.
export function entryKindForContentType(ct: ContentType): string {
  switch (ct) {
    case 'music':  return 'artist'
    case 'adult':
    case 'jav':    return 'studio'
    case 'tv':     return 'series'
    case 'movie':  return 'movie'
    case 'book':   return 'author'
  }
}

export interface ImportEntryResult {
  entry: LibraryEntry
  network?: LibraryEntry
}

export function importEntry(req: ImportEntryRequest) {
  return post<ImportEntryResult>('/metadata/entries/import', req)
}

// ── Import person ─────────────────────────────────────────────────────────────

export interface ImportPersonRequest {
  source: string
  externalId: string
  name: string
  aliases?: string[]
  overview?: string
  role?: PersonRole
  monitored: boolean
  monitorMode: MonitorMode
  metadata?: Record<string, unknown>
}

export function importPerson(req: ImportPersonRequest) {
  return post<Person>('/metadata/people/import', req)
}

// ── Discography ───────────────────────────────────────────────────────────────

export interface DiscographyResult {
  results: ExternalGroup[]
  total: number
  page: number
  pageSize: number
}

export function fetchArtistDiscography(source: string, contentType: string, externalId: string, page = 1, pageSize = 50) {
  return get<DiscographyResult>('/metadata/discography', { source, contentType, externalId, page, pageSize })
}

// ── Import album ──────────────────────────────────────────────────────────────

export interface ImportAlbumRequest {
  source: string
  externalId: string
  libraryEntryId: string
  title: string
  year?: number
  monitored: boolean
  monitorMode: MonitorMode
  primaryType?: string
  secondaryTypes?: string[]
}

export function importAlbum(req: ImportAlbumRequest) {
  return post<Group>('/metadata/albums/import', req)
}

// ── Track search ──────────────────────────────────────────────────────────────

export function searchTracks(source: string, contentType: ContentType, groupExternalId: string, q?: string, limit = 200) {
  return get<{ results: ExternalTrack[] }>('/metadata/search', {
    kind: 'track',
    source,
    contentType,
    groupExternalId,
    ...(q ? { q } : {}),
    limit,
  })
}

// ── Import item ───────────────────────────────────────────────────────────────

export interface ImportItemRequest {
  source: string
  externalId: string
  contentType: ContentType
  albumExternalId?: string
  albumTitle?: string
  monitored: boolean
}

export interface ImportItemResult {
  item: Item
  entry?: LibraryEntry
  network?: LibraryEntry
  album?: Group
}

export interface ImportItemJobAccepted {
  job_id: string
}

export function importItem(req: ImportItemRequest) {
  return post<ImportItemResult | ImportItemJobAccepted>('/metadata/items/import', req)
}
