import { describe, expect, it } from 'vitest'
import { buildImportItemParamsForFile } from './AlbumUnmatchedGroup'
import type { UnmatchedFile } from '../../types'

function makeFile(overrides: Partial<UnmatchedFile> = {}): UnmatchedFile {
  return {
    id: 'uf-1',
    path: '/music/track.flac',
    size: 12345,
    content_type: 'music',
    discovered_at: '2026-01-01T00:00:00Z',
    status: 'pending',
    candidates: [],
    ...overrides,
  }
}

describe('buildImportItemParamsForFile', () => {
  it('returns null when file has no candidates', () => {
    expect(buildImportItemParamsForFile(makeFile())).toBeNull()
  })

  it('returns null when best candidate has no external', () => {
    const f = makeFile({
      candidates: [{ item_id: 'item-1', confidence: 0.5, source: 'filename', source_label: 'Filename' }],
    })
    expect(buildImportItemParamsForFile(f)).toBeNull()
  })

  it('returns null when external has no external_id', () => {
    const f = makeFile({
      candidates: [{
        item_id: '',
        confidence: 0.5,
        source: 'filename',
        source_label: 'Filename',
        external: { source: 'musicbrainz', external_id: '', title: 'Track', content_type: 'music' },
      }],
    })
    expect(buildImportItemParamsForFile(f)).toBeNull()
  })

  it('returns correct params with group_external_id from candidate', () => {
    const f = makeFile({
      candidates: [{
        item_id: '',
        confidence: 0.85,
        source: 'acoustid',
        source_label: 'Audio Fingerprint',
        external: {
          source: 'musicbrainz',
          external_id: 'rec-abc',
          title: 'Track A',
          content_type: 'music',
          group_external_id: 'release-xyz',
        },
      }],
    })
    expect(buildImportItemParamsForFile(f)).toEqual({
      source: 'musicbrainz',
      externalId: 'rec-abc',
      contentType: 'music',
      monitored: true,
      albumExternalId: 'release-xyz',
    })
  })

  it('falls back to embedded_tags musicbrainz_release_id when candidate has no group_external_id', () => {
    const f = makeFile({
      fingerprint: { embedded_tags: { musicbrainz_release_id: 'rel-123', album: 'My Album' } },
      candidates: [{
        item_id: '',
        confidence: 0.75,
        source: 'tags',
        source_label: 'Embedded Tags',
        external: { source: 'musicbrainz', external_id: 'rec-def', title: 'Track B', content_type: 'music' },
      }],
    })
    const params = buildImportItemParamsForFile(f)
    expect(params?.albumExternalId).toBe('rel-123')
    expect(params?.albumTitle).toBe('My Album')
  })

  it('falls back to embedded_tags musicbrainz_album_id when release_id absent', () => {
    const f = makeFile({
      fingerprint: { embedded_tags: { musicbrainz_album_id: 'alb-456' } },
      candidates: [{
        item_id: '',
        confidence: 0.70,
        source: 'tags',
        source_label: 'Embedded Tags',
        external: { source: 'musicbrainz', external_id: 'rec-ghi', title: 'Track C', content_type: 'music' },
      }],
    })
    expect(buildImportItemParamsForFile(f)?.albumExternalId).toBe('alb-456')
  })

  it('omits album fields when no album info is available', () => {
    const f = makeFile({
      candidates: [{
        item_id: '',
        confidence: 0.50,
        source: 'filename',
        source_label: 'Filename',
        external: { source: 'musicbrainz', external_id: 'rec-jkl', title: 'Track D', content_type: 'music' },
      }],
    })
    const params = buildImportItemParamsForFile(f)
    expect(params?.albumExternalId).toBeUndefined()
    expect(params?.albumTitle).toBeUndefined()
  })
})
