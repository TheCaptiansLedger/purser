import { describe, expect, it } from 'vitest'
import { sourceBadgeLabel } from './ImageSelector'

// BannerPicker reuses sourceBadgeLabel for thumbnail source badges.
// These tests verify the shared label mapping used in both components.
describe('sourceBadgeLabel (used by BannerPicker thumbnails)', () => {
  it('maps known provider sources', () => {
    expect(sourceBadgeLabel('theaudiodb')).toBe('AudioDB')
    expect(sourceBadgeLabel('fanart')).toBe('Fanart')
    expect(sourceBadgeLabel('tmdb')).toBe('TMDB')
  })

  it('passes through unknown sources unchanged', () => {
    expect(sourceBadgeLabel('myprovider')).toBe('myprovider')
  })
})
