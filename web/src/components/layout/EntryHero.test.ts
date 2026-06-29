import { describe, expect, it } from 'vitest'
import { resolveHeroBackdrop } from './EntryHero'

describe('resolveHeroBackdrop', () => {
  it('uses banner and disables blur when banner is set', () => {
    const result = resolveHeroBackdrop('https://cdn.example.com/banner.jpg?v=2', 'https://cdn.example.com/image.jpg?v=1')
    expect(result).toEqual({ url: 'https://cdn.example.com/banner.jpg?v=2', blur: false })
  })

  it('falls back to image url and enables blur when banner is absent', () => {
    const result = resolveHeroBackdrop(undefined, 'https://cdn.example.com/image.jpg?v=1')
    expect(result).toEqual({ url: 'https://cdn.example.com/image.jpg?v=1', blur: true })
  })

  it('returns undefined url and blurs when both are absent', () => {
    const result = resolveHeroBackdrop(undefined, undefined)
    expect(result).toEqual({ url: undefined, blur: true })
  })

  it('uses banner and ignores fallback when both are present', () => {
    const result = resolveHeroBackdrop('/api/v1/images/entry-banners/abc?v=3', '/api/v1/images/library-entries/abc?v=1')
    expect(result).toEqual({ url: '/api/v1/images/entry-banners/abc?v=3', blur: false })
  })
})
