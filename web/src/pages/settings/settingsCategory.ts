import type { Setting, SettingCategory } from '../../types'

interface CategoryDef {
  category: SettingCategory
  label: string
  prefixes: string[]
}

// CATEGORY_DEFS is a registry, not a hardcoded switch — mirrors the
// NAV_ITEMS/TAB_ITEMS pattern used elsewhere in this app. Order here is
// the Config tab's card display order.
//
// This is a UI-only grouping by dotted-key prefix; neither the proto nor
// SettingsService models "category" — GetSettings returns one flat list
// (see web/src/types/index.ts's SettingCategory doc comment).
//
// database. is the only prefix internal/config/overlay.go structurally
// excludes from the DB-overlay layer (bootstrap-locked, unconditionally —
// docs/adr/0028-layered-settings.md). server./paths./log./telemetry. are
// ordinary DB-overlay-eligible keys folded into one "Server" card for
// display purposes only; they render locked only when GetSettings
// actually reports them operator-locked (env/yaml), same as any other
// card — grouping them together does not imply they're always locked.
const CATEGORY_DEFS: CategoryDef[] = [
  { category: 'server', label: 'Server', prefixes: ['server.', 'paths.', 'log.', 'telemetry.'] },
  { category: 'database', label: 'Database', prefixes: ['database.'] },
  { category: 'media', label: 'Media', prefixes: ['media.'] },
  { category: 'pipeline', label: 'Pipeline', prefixes: ['pipeline.'] },
  { category: 'sources', label: 'Sources', prefixes: ['sources.', 'musicbrainz.', 'acoustid.'] },
  { category: 'afterdark', label: 'AfterDark', prefixes: ['afterdark.'] },
  {
    category: 'downloadClients',
    label: 'Prowlarr / QBittorrent / SABnzbd',
    prefixes: ['prowlarr.', 'qbittorrent.', 'sabnzbd.'],
  },
  { category: 'modules', label: 'Modules', prefixes: ['modules.'] },
]

export const CATEGORY_ORDER: SettingCategory[] = CATEGORY_DEFS.map(d => d.category)

// categoryOf returns key's card category, or undefined for a key with no
// registered prefix — a future internal/config key added without a
// matching entry above, which groupSettingsByCategory below drops rather
// than crashing on.
export function categoryOf(key: string): SettingCategory | undefined {
  return CATEGORY_DEFS.find(def => def.prefixes.some(prefix => key.startsWith(prefix)))?.category
}

export function categoryLabel(category: SettingCategory): string {
  return CATEGORY_DEFS.find(def => def.category === category)?.label ?? category
}

// groupSettingsByCategory buckets settings into every known category, in
// CATEGORY_ORDER, dropping any key whose prefix isn't registered above
// instead of throwing — the Config tab must stay renderable even if
// internal/config's schema outpaces this file.
export function groupSettingsByCategory(settings: Setting[]): Map<SettingCategory, Setting[]> {
  const groups = new Map<SettingCategory, Setting[]>(CATEGORY_ORDER.map(category => [category, []]))
  for (const setting of settings) {
    const category = categoryOf(setting.key)
    if (!category) continue
    groups.get(category)!.push(setting)
  }
  return groups
}
