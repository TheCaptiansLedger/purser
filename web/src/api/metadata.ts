import { get, post } from './client'
import type { ContentType, ExternalStudio, ExternalPerson, LibraryEntry, MonitorMode, Person, PersonRole } from '../types'

// ── Search ────────────────────────────────────────────────────────────────────

export function searchStudios(q: string, contentType?: ContentType, limit = 25) {
  return get<{ results: ExternalStudio[] }>('/metadata/search', { kind: 'studio', q, contentType, limit })
}

export function searchPeople(q: string, contentType?: ContentType, limit = 25) {
  return get<{ results: ExternalPerson[] }>('/metadata/search', { kind: 'person', q, contentType, limit })
}

// ── Import studio ─────────────────────────────────────────────────────────────

export interface ImportStudioRequest {
  source: string
  externalId: string
  name: string
  overview?: string
  contentType: ContentType
  monitored: boolean
  monitorMode: MonitorMode
  parentExternalId?: string
  parentName?: string
  parentImageUrl?: string
  parentWebsiteUrl?: string
  imageUrl?: string
  websiteUrl?: string
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
