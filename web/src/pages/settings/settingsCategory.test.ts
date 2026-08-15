import { describe, expect, it } from 'vitest'
import type { Setting } from '../../types'
import { categoryLabel, categoryOf, groupSettingsByCategory } from './settingsCategory'

function setting(key: string): Setting {
  return { key, value: 'null', source: 'default', locked: false, lockReason: 'none', secret: false }
}

describe('categoryOf', () => {
  it('maps server/paths/log/telemetry keys to the server category', () => {
    expect(categoryOf('server.listen_addr')).toBe('server')
    expect(categoryOf('paths.data_dir')).toBe('server')
    expect(categoryOf('log.level')).toBe('server')
    expect(categoryOf('telemetry.enabled')).toBe('server')
  })

  it('groups by owning module, not by dotted prefix — AfterDark example', () => {
    // None of these three share a dotted prefix with each other, but all
    // three are AfterDark's own settings (settingFields.ts's per-key
    // category override, not settingsCategory.ts's prefix fallback).
    expect(categoryOf('sources.stashdb.api_key')).toBe('afterdark')
    expect(categoryOf('afterdark.provider_priority')).toBe('afterdark')
    expect(categoryOf('pipeline.organize.adult.template')).toBe('afterdark')
  })

  it('groups by owning module, not by dotted prefix — Music example', () => {
    expect(categoryOf('musicbrainz.base_url')).toBe('music')
    expect(categoryOf('acoustid.api_key')).toBe('music')
    expect(categoryOf('sources.theaudiodb.api_key')).toBe('music')
    expect(categoryOf('sources.fanart.api_key')).toBe('music')
    expect(categoryOf('pipeline.organize.music.template')).toBe('music')
  })

  it('falls back to the module prefix for a key with no per-key override', () => {
    expect(categoryOf('modules.movies.enabled')).toBe('movies')
    expect(categoryOf('modules.tv.roots')).toBe('tv')
    expect(categoryOf('modules.books.enabled')).toBe('books')
  })

  it('returns undefined for an unregistered prefix', () => {
    expect(categoryOf('unknown.key')).toBeUndefined()
  })
})

describe('categoryLabel', () => {
  it('labels the combined download-clients category by product names', () => {
    expect(categoryLabel('downloadClients')).toBe('Prowlarr / QBittorrent / SABnzbd')
  })

  it('labels a single-prefix category by its own name', () => {
    expect(categoryLabel('database')).toBe('Database')
  })
})

describe('groupSettingsByCategory', () => {
  it('buckets every known category, including ones with no settings', () => {
    const groups = groupSettingsByCategory([setting('database.driver')])
    expect(groups.get('database')).toHaveLength(1)
    expect(groups.get('movies')).toEqual([])
  })

  it('drops a setting whose key has no registered prefix', () => {
    const groups = groupSettingsByCategory([setting('unknown.key'), setting('media.path')])
    expect(groups.get('media')).toHaveLength(1)
    expect([...groups.values()].flat()).toHaveLength(1)
  })

  it('buckets AfterDark keys from three different dotted prefixes onto one card', () => {
    const groups = groupSettingsByCategory([
      setting('sources.stashdb.api_key'),
      setting('afterdark.provider_priority'),
      setting('pipeline.organize.adult.template'),
      setting('modules.afterdark.enabled'),
    ])
    expect(groups.get('afterdark')).toHaveLength(4)
  })
})
