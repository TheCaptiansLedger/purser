import { describe, expect, it } from 'vitest'
import {
  fieldCategory,
  fieldInputKind,
  fieldLabel,
  fieldOptions,
  fieldTemplateFields,
  fieldUnit,
} from './settingFields'

describe('fieldLabel', () => {
  it('returns the registered human label for a known key', () => {
    expect(fieldLabel('server.listen_addr')).toBe('Listen Address')
    expect(fieldLabel('pipeline.organize.adult.template')).toBe('Rename Template')
  })

  it('falls back to a prettified last segment for an unregistered key', () => {
    expect(fieldLabel('sources.newprovider.response_header_timeout')).toBe('Response Header Timeout')
  })
})

describe('fieldCategory', () => {
  it('overrides an afterdark-owned key that shares no dotted prefix with afterdark.*', () => {
    expect(fieldCategory('pipeline.organize.adult.template')).toBe('afterdark')
    expect(fieldCategory('sources.stashdb.api_key')).toBe('afterdark')
  })

  it('overrides a music-owned key the same way', () => {
    expect(fieldCategory('musicbrainz.base_url')).toBe('music')
    expect(fieldCategory('sources.fanart.api_key')).toBe('music')
  })

  it('returns undefined for a key with no override, leaving prefix fallback to settingsCategory.ts', () => {
    expect(fieldCategory('server.listen_addr')).toBeUndefined()
  })
})

describe('fieldUnit', () => {
  it('returns a unit hint for a duration/ratio field', () => {
    expect(fieldUnit('pipeline.confidence_threshold')).toBe('0.0–1.0')
    expect(fieldUnit('musicbrainz.response_header_timeout')).toContain('45s')
  })

  it('returns undefined for a field with no unit', () => {
    expect(fieldUnit('server.listen_addr')).toBeUndefined()
  })
})

describe('fieldInputKind / fieldOptions', () => {
  it('marks pipeline.scan_roots for the struct-array editor', () => {
    expect(fieldInputKind('pipeline.scan_roots')).toBe('scanRoots')
  })

  it('marks afterdark.provider_priority for the priority-list editor with its closed option set', () => {
    expect(fieldInputKind('afterdark.provider_priority')).toBe('priorityList')
    expect(fieldOptions('afterdark.provider_priority')).toEqual([
      { value: 'stashdb', label: 'StashDB' },
      { value: 'tpdb', label: 'ThePornDB' },
    ])
  })
})

describe('fieldTemplateFields', () => {
  it('documents AfterDark and Music rename-template fields independently', () => {
    const afterdark = fieldTemplateFields('pipeline.organize.adult.template')
    const music = fieldTemplateFields('pipeline.organize.music.template')
    expect(afterdark?.map(f => f.name)).toContain('SceneTitle')
    expect(music?.map(f => f.name)).toContain('AlbumTitle')
    expect(afterdark).not.toEqual(music)
  })

  it('returns undefined for a field with no template documentation', () => {
    expect(fieldTemplateFields('server.listen_addr')).toBeUndefined()
  })
})
