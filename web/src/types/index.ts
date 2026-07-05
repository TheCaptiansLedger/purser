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
  result?: Record<string, unknown>
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

export interface PersonRoleCount {
  role: PersonRole
  count: number
}

export interface ContentTypeConfig {
  contentType: ContentType
  label: string
  moduleKey: string
  personRoles: string[]
  supportsFileGrouping: boolean
}

export interface KindConfig {
  kind: Kind
  personRoles: string[]
  showDates: boolean
}

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
  primaryType?: string
  secondaryTypes?: string[]
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

export interface Tag {
  id: string
  key: string
  value: string
  scope: 'user' | 'metadata'
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
  md5?: string
  quality: string
  resolution: string
  codec: string
  container: string
  addedAt: string
}

export interface ExternalTrack {
  source: string
  externalId: string
  title: string
  sequence?: string
  runtimeSeconds?: number
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
  roles: PersonRole[]
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
  bannerUrl?: string
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
  externalIds: ExternalID[]
  tags: Tag[]
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


export interface DeletionImpactRow {
  kind: string
  count: number
  label: string
}

export interface DeletionImpact {
  mode: 'destroy' | 'unlink'
  summary: string
  impacts: DeletionImpactRow[]
}

export interface Fingerprint {
  oshash?: string
  phash?: string
  acoust_id?: string
  embedded_tags?: Record<string, string>
  isbn?: string
}

export interface ExternalParent {
  source: string
  external_id: string
  name: string
  parent_id?: string
  parent_name?: string
  image_url?: string
  parent_image_url?: string
}

export interface ExternalCandidateItem {
  source: string
  external_id: string
  title: string
  content_type: string
  parent_kind?: string
  image_url?: string
  overview?: string
  date?: string
  runtime_seconds?: number
  group_external_id?: string
  group_title?: string
  parent?: ExternalParent
}

export interface MatchCandidate {
  item_id: string
  item_title?: string
  confidence: number
  source: string
  source_label: string
  source_description?: string
  external?: ExternalCandidateItem
}

export type UnmatchedStatus = 'pending' | 'matched' | 'dismissed'

export interface UnmatchedFile {
  id: string
  path: string
  size: number
  content_type: string
  discovered_at: string
  status: UnmatchedStatus
  fingerprint?: Fingerprint
  candidates: MatchCandidate[]
}

export interface UnmatchedFileGroup {
  group_id: string
  group_title: string
  files: UnmatchedFile[]
  best_candidate_confidence: number
  cover_url?: string
}

export interface UnmatchedListResponse<T = UnmatchedFile | UnmatchedFileGroup> {
  items: T[]
  total: number
  grouped_by?: string
}
