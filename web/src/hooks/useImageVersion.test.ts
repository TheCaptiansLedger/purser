import { describe, expect, it } from 'vitest'
import { buildVersionedUrl } from './useImageVersion'

describe('buildVersionedUrl', () => {
  it('returns undefined when baseUrl is undefined', () => {
    expect(buildVersionedUrl(undefined, 0)).toBeUndefined()
    expect(buildVersionedUrl(undefined, 5)).toBeUndefined()
  })

  it('appends ?v=0 on the initial version', () => {
    expect(buildVersionedUrl('http://localhost/img.jpg', 0)).toBe('http://localhost/img.jpg?v=0')
  })

  it('appends the correct version after bumps', () => {
    expect(buildVersionedUrl('http://localhost/img.jpg', 3)).toBe('http://localhost/img.jpg?v=3')
  })
})
