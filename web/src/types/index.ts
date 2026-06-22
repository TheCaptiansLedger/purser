export interface ProviderImage {
  url: string
  type: string
  source: string
  width: number
  height: number
}

export interface Page<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

export type JobStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface Job {
  id: string
  name: string
  payload?: Record<string, unknown>
  status: JobStatus
  current: number
  total: number
  message?: string
  error?: string
  createdAt: string
  startedAt?: string
  completedAt?: string
}

export type ContentType = 'movie' | 'tv' | 'music' | 'adult' | 'jav' | 'book'
export type Kind = 'network' | 'studio' | 'series' | 'artist' | 'movie' | 'publisher' | 'book'
export type MonitorMode = 'all' | 'future' | 'none' | 'latest'
export type EntryStatus = 'continuing' | 'ended' | 'active'
export type ItemStatus = 'wanted' | 'grabbed' | 'downloading' | 'imported' | 'missing' | 'skipped'
export type PersonRole = 'performer' | 'actress' | 'director' | 'actor' | 'artist' | 'producer' | 'author'

export interface ExternalID {
  source: string
  value: string
}

export interface ExternalStudio {
  source: string
  externalId: string
  name: string
  overview?: string
  imageUrl?: string
  websiteUrl?: string
  parentExternalId?: string
  parentName?: string
  parentImageUrl?: string
  parentWebsiteUrl?: string
}

export interface ExternalGroup {
  source: string
  externalId: string
  title: string
  year?: number
}

export interface ExternalPerson {
  source: string
  externalId: string
  name: string
  aliases?: string[]
  overview?: string
  imageUrl?: string
  role?: PersonRole
  metadata?: Record<string, unknown>
}

export type TagCategory = '' | 'genre' | 'content_warning'

export interface Tag {
  id: string
  name: string
  scope: 'user' | 'metadata'
  category: TagCategory
}

export interface PersonRef {
  id: string
  name: string
  sortName: string
  imageUrl?: string
}

export interface ItemPerson {
  personId: string
  person?: PersonRef
  role: PersonRole
}

export interface EntryPerson {
  personId: string
  person?: PersonRef
  role: string
  startDate?: string
  endDate?: string
}

export interface MediaFile {
  id: string
  path: string
  size: number
  osHash: string
  quality: string
  resolution: string
  codec: string
  container: string
  addedAt: string
}

export interface Person {
  id: string
  name: string
  sortName: string
  overview: string
  monitored: boolean
  monitorMode: MonitorMode
  imageUrl?: string
  aliases: string[]
  externalIds: ExternalID[]
  metadata?: Record<string, unknown>
  lockedFields?: string[]
  addedAt: string
}

export interface LibraryEntry {
  id: string
  contentType: ContentType
  kind: Kind
  name: string
  sortName: string
  overview: string
  parentId?: string
  monitored: boolean
  monitorMode: MonitorMode
  status: EntryStatus
  qualityProfileId?: string
  metadataProfileId?: string
  path?: string
  imageUrl?: string
  externalIds: ExternalID[]
  tags: Tag[]
  people: EntryPerson[]
  metadata?: Record<string, unknown>
  lockedFields?: string[]
  addedAt: string
  updatedAt: string
}

export interface Group {
  id: string
  libraryEntryId: string
  title: string
  sortName: string
  number: number
  year: number
  overview: string
  monitored: boolean
  monitorMode: MonitorMode
  coverUrl?: string
  metadata?: Record<string, unknown>
  lockedFields?: string[]
}

export interface Item {
  id: string
  contentType: ContentType
  libraryEntryId: string
  groupId?: string
  title: string
  overview: string
  date?: string
  sequence?: string
  runtimeSeconds: number
  monitored: boolean
  status: ItemStatus
  coverUrl?: string
  people: ItemPerson[]
  tags: Tag[]
  externalIds: ExternalID[]
  mediaFile?: MediaFile
  metadata?: Record<string, unknown>
  lockedFields?: string[]
  addedAt: string
  updatedAt: string
}
