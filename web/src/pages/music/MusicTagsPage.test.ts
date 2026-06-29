import { describe, it, expect } from 'vitest'
import { getTagDisplayText, getTagHref } from './MusicTagsPage'

describe('getTagDisplayText', () => {
  it('returns bare value for label (VIP key)', () => {
    expect(getTagDisplayText('label', 'Atlantic Records')).toBe('Atlantic Records')
  })

  it('returns bare value for genre (VIP key)', () => {
    expect(getTagDisplayText('genre', 'Jazz')).toBe('Jazz')
  })

  it('returns key:value for non-VIP keys', () => {
    expect(getTagDisplayText('fuzzy', 'bunny')).toBe('fuzzy:bunny')
    expect(getTagDisplayText('mood', 'melancholy')).toBe('mood:melancholy')
  })
})

describe('getTagHref', () => {
  it('routes label tags to /music/labels/:value', () => {
    expect(getTagHref('label', 'Atlantic Records')).toBe('/music/labels/Atlantic%20Records')
  })

  it('routes genre tags to /tags/:key/:value', () => {
    expect(getTagHref('genre', 'Jazz')).toBe('/tags/genre/Jazz')
  })

  it('routes non-VIP tags to /tags/:key/:value', () => {
    expect(getTagHref('fuzzy', 'bunny')).toBe('/tags/fuzzy/bunny')
  })

  it('URL-encodes special characters in label values', () => {
    expect(getTagHref('label', 'A&M Records')).toBe('/music/labels/A%26M%20Records')
  })

  it('URL-encodes special characters in non-label values', () => {
    expect(getTagHref('mood', 'lo-fi/chill')).toBe('/tags/mood/lo-fi%2Fchill')
  })
})
