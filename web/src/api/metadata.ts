import { get, post } from './client'
import type { ContentType, ExternalGroup, ExternalStudio, ExternalPerson, Group, LibraryEntry, MonitorMode, Person, PersonRole } from '../types'

// ── Search ────────────────────────────────────────────────────────────────────

export function searchStudios(q: string, contentType?: ContentType, limit = 25) {
  return get<{ results: ExternalStudio[] }>('/metadata/search', { kind: 'studio', q, contentType, limit })
}

export function searchPeople(q: string, contentType?: ContentType, limit = 25) {
  return get<{ results: ExternalPerson[] }>('/metadata/search', { kind: 'person', q, contentType, limit })
}

// ── Import studio ─────────────────────────────────────────────────────────────

export type AlbumFilterToken = 'studio' | 'live' | 'compilation' | 'ep' | 'single' | 'all'

export interface ImportStudioRequest {
  source: string
  externalId: string
  name: string
  overview?: string
  contentType: ContentType
  kind?: string
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

export interface ImportStudioResult {
  studio: LibraryEntry
  network?: LibraryEntry
}

export function importStudio(req: ImportStudioRequest) {
  return post<ImportStudioResult>('/metadata/studios/import', req)
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

export function fetchArtistDiscography(source: string, externalId: string, page = 1, pageSize = 50) {
  return get<DiscographyResult>('/metadata/discography', { source, externalId, page, pageSize })
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
}

export function importAlbum(req: ImportAlbumRequest) {
  return post<Group>('/metadata/albums/import', req)
}
