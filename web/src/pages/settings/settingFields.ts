import type { SettingCategory } from '../../types'

// TemplateFieldDoc documents one variable a rename template can reference
// — {name, example} pairs shown by InfoPopover. Hand-maintained, not
// derived from Go source: mirrors internal/adapters/pipeline/<module>/
// template_data_builder.go's BuildTemplateData return map (plus Ext,
// which internal/service/organizer.go adds generically for every content
// type). Update here if a TemplateDataBuilder's field set changes.
export interface TemplateFieldDoc {
  name: string
  example: string
}

// AfterDark's TemplateDataBuilder — see
// internal/adapters/pipeline/afterdark/template_data_builder.go.
const AFTERDARK_TEMPLATE_FIELDS: TemplateFieldDoc[] = [
  { name: 'Studio', example: 'Studio Name' },
  { name: 'Performers', example: 'Jane Doe, John Smith' },
  { name: 'SceneTitle', example: 'Scene Title' },
  { name: 'SceneDate', example: '2024-03-15' },
  { name: 'SceneCode', example: 'ABC-123' },
  { name: 'Ext', example: '.mp4' },
]

// Music's TemplateDataBuilder — see
// internal/adapters/pipeline/music/template_data_builder.go.
const MUSIC_TEMPLATE_FIELDS: TemplateFieldDoc[] = [
  { name: 'ArtistName', example: 'Artist Name' },
  { name: 'AlbumTitle', example: 'Album Title' },
  { name: 'TrackTitle', example: 'Track Title' },
  { name: 'TrackNumber', example: '7' },
  { name: 'DiscNumber', example: '1' },
  { name: 'DiscCount', example: '2' },
  { name: 'Year', example: '2024' },
  { name: 'Ext', example: '.flac' },
]

// SettingInputKind picks a bespoke field component over the generic
// type-based dispatch in SettingsCard.tsx's SettingInput — for the two
// keys whose shape ListInput can't represent: pipeline.scan_roots is a
// struct array (ListInput assumes string[] and would render
// "[object Object]"), and afterdark.provider_priority is a closed,
// order-sensitive set (ListInput's free-text add lets an operator type
// anything, and a flat tag list has no notion of rank).
export type SettingInputKind = 'scanRoots' | 'priorityList'

export interface SettingOption {
  value: string
  label: string
}

export interface SettingFieldDef {
  key: string
  label: string
  // category overrides settingsCategory.ts's prefix-based fallback — the
  // mechanism that lets e.g. pipeline.organize.adult.template and
  // sources.stashdb.api_key land on the same AfterDark card despite
  // sharing no dotted prefix.
  category?: SettingCategory
  // unit is a short hint appended after the label (e.g. "e.g. 45s") for
  // any field whose bare value doesn't self-describe its unit.
  unit?: string
  input?: SettingInputKind
  // options is priorityList's closed value set — the only valid entries
  // an operator can reorder into afterdark.provider_priority.
  options?: SettingOption[]
  templateFields?: TemplateFieldDoc[]
}

// FIELD_DEFS is a registry, not a hardcoded switch — same
// hand-maintained-by-exact-key pattern settingsCategory.ts's CATEGORY_DEFS
// already uses. A key with no entry here falls back to a prettified
// version of its own dotted key (see fieldLabel) and to
// settingsCategory.ts's prefix-based category — nothing crashes or
// disappears if internal/config grows a field this file doesn't know
// about yet.
const FIELD_DEFS: SettingFieldDef[] = [
  // Server
  { key: 'server.listen_addr', label: 'Listen Address' },
  { key: 'paths.data_dir', label: 'Data Directory' },
  { key: 'log.level', label: 'Log Level' },
  { key: 'log.format', label: 'Log Format' },
  { key: 'telemetry.enabled', label: 'Telemetry Enabled' },
  { key: 'telemetry.otlp_endpoint', label: 'OTLP Endpoint' },
  { key: 'telemetry.otlp_insecure', label: 'Allow Insecure OTLP' },
  { key: 'telemetry.metrics_addr', label: 'Metrics Address' },

  // Database (always locked, read-only — labels still matter for display)
  { key: 'database.driver', label: 'Driver' },
  { key: 'database.badger.data_dir', label: 'Badger Data Directory' },
  { key: 'database.badger.value_log_dir', label: 'Badger Value Log Directory' },
  { key: 'database.badger.sync_writes', label: 'Sync Writes' },
  { key: 'database.sql.dsn', label: 'Connection String' },

  // Media
  { key: 'media.path', label: 'Media Path' },

  // Pipeline (core scan behavior only — module-specific organize keys
  // below are re-categorized onto their owning module's card)
  { key: 'pipeline.enable_md5', label: 'Compute MD5 Hash' },
  { key: 'pipeline.enable_sha512', label: 'Compute SHA-512 Hash' },
  { key: 'pipeline.scan_roots', label: 'Scan Roots', input: 'scanRoots' },
  { key: 'pipeline.confidence_threshold', label: 'Match Confidence Threshold', unit: '0.0–1.0' },

  // Music module
  { key: 'modules.music.enabled', label: 'Enabled', category: 'music' },
  { key: 'modules.music.roots', label: 'Library Roots', category: 'music' },
  { key: 'pipeline.organize.music.root', label: 'Organize Root', category: 'music' },
  {
    key: 'pipeline.organize.music.template',
    label: 'Rename Template',
    category: 'music',
    templateFields: MUSIC_TEMPLATE_FIELDS,
  },
  { key: 'musicbrainz.base_url', label: 'MusicBrainz Base URL', category: 'music' },
  {
    key: 'musicbrainz.response_header_timeout',
    label: 'MusicBrainz Response Timeout',
    category: 'music',
    unit: 'Go duration, e.g. 45s',
  },
  { key: 'acoustid.base_url', label: 'AcoustID Base URL', category: 'music' },
  { key: 'acoustid.api_key', label: 'AcoustID API Key', category: 'music' },
  { key: 'sources.theaudiodb.enabled', label: 'TheAudioDB Enabled', category: 'music' },
  { key: 'sources.theaudiodb.api_key', label: 'TheAudioDB API Key', category: 'music' },
  { key: 'sources.fanart.enabled', label: 'fanart.tv Enabled', category: 'music' },
  { key: 'sources.fanart.api_key', label: 'fanart.tv API Key', category: 'music' },

  // AfterDark module
  { key: 'modules.afterdark.enabled', label: 'Enabled', category: 'afterdark' },
  { key: 'modules.afterdark.roots', label: 'Library Roots', category: 'afterdark' },
  { key: 'pipeline.organize.adult.root', label: 'Organize Root', category: 'afterdark' },
  {
    key: 'pipeline.organize.adult.template',
    label: 'Rename Template',
    category: 'afterdark',
    templateFields: AFTERDARK_TEMPLATE_FIELDS,
  },
  {
    key: 'afterdark.provider_priority',
    label: 'Provider Priority',
    category: 'afterdark',
    input: 'priorityList',
    options: [
      { value: 'stashdb', label: 'StashDB' },
      { value: 'tpdb', label: 'ThePornDB' },
    ],
  },
  { key: 'sources.stashdb.enabled', label: 'StashDB Enabled', category: 'afterdark' },
  { key: 'sources.stashdb.api_key', label: 'StashDB API Key', category: 'afterdark' },
  { key: 'sources.tpdb.enabled', label: 'ThePornDB Enabled', category: 'afterdark' },
  { key: 'sources.tpdb.api_key', label: 'ThePornDB API Key', category: 'afterdark' },

  // Movies / TV / Books modules — minimal today (no dedicated pipeline
  // wiring beyond enable/roots exists yet); organize.* entries pick up
  // automatically once those content types get one, no code change here.
  { key: 'modules.movies.enabled', label: 'Enabled', category: 'movies' },
  { key: 'modules.movies.roots', label: 'Library Roots', category: 'movies' },
  { key: 'pipeline.organize.movie.root', label: 'Organize Root', category: 'movies' },
  { key: 'pipeline.organize.movie.template', label: 'Rename Template', category: 'movies' },
  { key: 'modules.tv.enabled', label: 'Enabled', category: 'tv' },
  { key: 'modules.tv.roots', label: 'Library Roots', category: 'tv' },
  { key: 'pipeline.organize.tv.root', label: 'Organize Root', category: 'tv' },
  { key: 'pipeline.organize.tv.template', label: 'Rename Template', category: 'tv' },
  { key: 'modules.books.enabled', label: 'Enabled', category: 'books' },
  { key: 'modules.books.roots', label: 'Library Roots', category: 'books' },
  { key: 'pipeline.organize.book.root', label: 'Organize Root', category: 'books' },
  { key: 'pipeline.organize.book.template', label: 'Rename Template', category: 'books' },

  // Download clients — combined card, so each label is product-prefixed
  // to stay unambiguous alongside its siblings.
  { key: 'prowlarr.enabled', label: 'Prowlarr Enabled' },
  { key: 'prowlarr.base_url', label: 'Prowlarr Base URL' },
  { key: 'prowlarr.api_key', label: 'Prowlarr API Key' },
  { key: 'prowlarr.response_header_timeout', label: 'Prowlarr Response Timeout', unit: 'Go duration, e.g. 45s' },
  { key: 'qbittorrent.enabled', label: 'QBittorrent Enabled' },
  { key: 'qbittorrent.base_url', label: 'QBittorrent Base URL' },
  { key: 'qbittorrent.username', label: 'QBittorrent Username' },
  { key: 'qbittorrent.password', label: 'QBittorrent Password' },
  { key: 'sabnzbd.enabled', label: 'SABnzbd Enabled' },
  { key: 'sabnzbd.base_url', label: 'SABnzbd Base URL' },
  { key: 'sabnzbd.api_key', label: 'SABnzbd API Key' },
]

const FIELD_BY_KEY = new Map(FIELD_DEFS.map(def => [def.key, def]))

// prettifyKey is the fallback label for any key with no registry entry —
// its last dotted segment, snake_case split into Title Case words (e.g.
// "response_header_timeout" -> "Response Header Timeout").
function prettifyKey(key: string): string {
  const segment = key.split('.').pop() ?? key
  return segment
    .split('_')
    .filter(Boolean)
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

export function fieldLabel(key: string): string {
  return FIELD_BY_KEY.get(key)?.label ?? prettifyKey(key)
}

export function fieldCategory(key: string): SettingCategory | undefined {
  return FIELD_BY_KEY.get(key)?.category
}

export function fieldUnit(key: string): string | undefined {
  return FIELD_BY_KEY.get(key)?.unit
}

export function fieldInputKind(key: string): SettingInputKind | undefined {
  return FIELD_BY_KEY.get(key)?.input
}

export function fieldOptions(key: string): SettingOption[] | undefined {
  return FIELD_BY_KEY.get(key)?.options
}

export function fieldTemplateFields(key: string): TemplateFieldDoc[] | undefined {
  return FIELD_BY_KEY.get(key)?.templateFields
}
