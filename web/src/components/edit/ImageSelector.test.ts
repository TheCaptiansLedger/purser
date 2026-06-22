import { describe, expect, it } from 'vitest'
import { sourceBadgeLabel } from './ImageSelector'

describe('sourceBadgeLabel', () => {
  it('maps known provider sources to display labels', () => {
    expect(sourceBadgeLabel('stashdb')).toBe('StashDB')
    expect(sourceBadgeLabel('fanart')).toBe('Fanart')
    expect(sourceBadgeLabel('tmdb')).toBe('TMDB')
    expect(sourceBadgeLabel('tvdb')).toBe('TVDB')
    expect(sourceBadgeLabel('theaudiodb')).toBe('AudioDB')
    expect(sourceBadgeLabel('musicbrainz')).toBe('MB')
  })

  it('returns the raw source string for unknown providers', () => {
    expect(sourceBadgeLabel('unknown')).toBe('unknown')
    expect(sourceBadgeLabel('someprovider')).toBe('someprovider')
  })
})
