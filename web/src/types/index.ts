// Hand-maintained types mirroring API responses — every field a component
// reads must exist here, with a test proving it (ADR 0004). Distinct from
// web/src/gen/**'s generated Message types: those are wire plumbing
// (protobuf oneof/method descriptors included), these are the stable
// surface app code is written against. Starts with Setting/settings-category
// types (#605) — extend per-entity as more of the app is built.

// SettingSource mirrors purser.settings.v1.SettingSource
// (web/src/gen/purser/settings/v1/settings_pb.ts) as plain string literals
// instead of the generated numeric enum, so a component branching on
// source doesn't need the proto import. See docs/adr/0028-layered-settings.md.
export type SettingSource = 'unspecified' | 'default' | 'env' | 'yaml' | 'db'

// SettingLockReason mirrors purser.settings.v1.SettingLockReason. 'none'
// stands in for SETTING_LOCK_REASON_UNSPECIFIED — Setting.locked is false
// whenever lockReason is 'none'.
export type SettingLockReason = 'none' | 'bootstrap' | 'operator'

// Setting mirrors purser.settings.v1.Setting. value is JSON-encoded,
// except when secret is true: value is then the literal masked placeholder
// ('********') when a value is set, or '' when unset — never
// JSON-encoded, never the real secret. See
// docs/adr/0028-layered-settings.md's "Secret masking" section.
export interface Setting {
  key: string
  value: string
  source: SettingSource
  locked: boolean
  lockReason: SettingLockReason
  secret: boolean
}

// SettingCategory groups Settings for the Config tab's card layout (#604)
// — a UI-only grouping by owning module, not modeled in the proto or
// SettingsService itself (GetSettings returns one flat list). Grouped by
// *product module*, not by the config file's top-level YAML section: e.g.
// sources.stashdb.* and pipeline.organize.adult.* both land under
// 'afterdark' despite neither sharing that dotted prefix. See
// web/src/pages/settings/settingFields.ts (per-key overrides) and
// web/src/pages/settings/settingsCategory.ts (prefix fallback + card
// labels/order).
export type SettingCategory =
  | 'server'
  | 'database'
  | 'media'
  | 'pipeline'
  | 'music'
  | 'afterdark'
  | 'movies'
  | 'tv'
  | 'books'
  | 'downloadClients'

// JobStatus mirrors purser.job.v1.JobStatus (web/src/gen/purser/job/v1/job_pb.ts)
// as plain string literals — see docs/adr/0023-job-queue.md. 'partial'
// applies only to a Job (some Tasks succeeded, some failed), never to a
// Task or Step.
export type JobStatus = 'unspecified' | 'pending' | 'running' | 'succeeded' | 'failed' | 'partial'

// Step mirrors purser.job.v1.Step — one discrete action taken on a Task
// (e.g. "compute hashes"). detail carries structured, step-specific
// results (a matched MBID, a computed confidence score) the job detail
// modal (#608) renders as a key/value list. See docs/adr/0023-job-queue.md.
export interface Step {
  id: string
  name: string
  status: JobStatus
  startedAt: Date | undefined
  finishedAt: Date | undefined
  message: string
  detail: Record<string, string>
}

// Task mirrors purser.job.v1.Task — one unit of work within a Job (one
// file, one track). progress is server-computed from its Steps, not
// derived here.
export interface Task {
  id: string
  label: string
  status: JobStatus
  startedAt: Date | undefined
  finishedAt: Date | undefined
  steps: Step[]
  progress: number
}

// Job mirrors purser.job.v1.Job. created/started/finishedAt are undefined
// when the wire Timestamp is unset (a pending Job has no startedAt; a
// running Job has no finishedAt), converted via @bufbuild/protobuf/wkt's
// timestampDate. progress is server-computed from the Job's Tasks, not
// derived here. tasks/params are the full Task/Step tree and the Job's
// trigger params, both unused by the Jobs tab (#606) table and read only
// by the job detail modal (#608).
export interface Job {
  id: string
  kind: string
  status: JobStatus
  createdAt: Date | undefined
  startedAt: Date | undefined
  finishedAt: Date | undefined
  progress: number
  tasks: Task[]
  params: Record<string, string>
}

// DatabaseInfo mirrors purser.database.v1.GetDatabaseInfoResponse (#613) —
// see docs/technical/database-backup-restore.md. storageSizeBytes and
// collectionCounts' values are wire int64 (bigint in the generated
// Message type); converted to number here since a real Purser install's
// storage size and per-collection counts stay comfortably inside
// Number.MAX_SAFE_INTEGER, and keeping components off bigint arithmetic
// is worth that ceiling.
export interface DatabaseInfo {
  driver: string
  version: string
  storageSizeBytes: number
  collectionCounts: Record<string, number>
}

// CacheStats mirrors the subset of purser.cache.v1.CacheStats the Cache
// tab (#617) actually renders (name/items/bytes/hits/misses). The wire
// message also carries sets/deletes/evictions — left off this type per
// ADR 0004 ("every field a component reads must exist here") until a
// component reads them; add them back the same way storageSizeBytes was
// added for DatabaseInfo if that changes. hits/misses/bytes are wire
// int64 (bigint); converted to number here for the same reason
// DatabaseInfo's counters are — comfortably inside
// Number.MAX_SAFE_INTEGER for a real cache, and it keeps components off
// bigint arithmetic.
export interface CacheStats {
  name: string
  items: number
  bytes: number
  hits: number
  misses: number
}
