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
// — a UI-only grouping by dotted-key prefix, not modeled in the proto or
// SettingsService itself (GetSettings returns one flat list). See
// web/src/pages/settings/settingsCategory.ts for the key->category mapping.
export type SettingCategory =
  | 'server'
  | 'database'
  | 'media'
  | 'pipeline'
  | 'sources'
  | 'afterdark'
  | 'downloadClients'
  | 'modules'
