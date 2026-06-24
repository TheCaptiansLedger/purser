import { describe, expect, it } from 'vitest'
import { canSearch } from './ImportDialog'

describe('canSearch', () => {
  it('returns false for empty string', () => {
    expect(canSearch('')).toBe(false)
  })

  it('returns false for whitespace-only', () => {
    expect(canSearch('  ')).toBe(false)
    expect(canSearch('\t')).toBe(false)
  })

  it('returns true for any non-empty trimmed string', () => {
    expect(canSearch('a')).toBe(true)
    expect(canSearch('Radiohead')).toBe(true)
    expect(canSearch(' padded ')).toBe(true)
  })
})
