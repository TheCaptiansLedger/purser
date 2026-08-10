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

  it('maps musicbrainz/acoustid keys into sources alongside sources.*', () => {
    expect(categoryOf('sources.stashdb.api_key')).toBe('sources')
    expect(categoryOf('musicbrainz.base_url')).toBe('sources')
    expect(categoryOf('acoustid.api_key')).toBe('sources')
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
    expect(groups.get('modules')).toEqual([])
  })

  it('drops a setting whose key has no registered prefix', () => {
    const groups = groupSettingsByCategory([setting('unknown.key'), setting('media.path')])
    expect(groups.get('media')).toHaveLength(1)
    expect([...groups.values()].flat()).toHaveLength(1)
  })
})
