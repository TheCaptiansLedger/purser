import { describe, expect, it } from 'vitest'
import { MIN_SEARCH_LEN } from './PersonSearchInput'

describe('MIN_SEARCH_LEN', () => {
  it('is 2', () => {
    expect(MIN_SEARCH_LEN).toBe(2)
  })

  it('gates search at the right threshold', () => {
    const shouldSearch = (q: string) => q.length >= MIN_SEARCH_LEN
    expect(shouldSearch('')).toBe(false)
    expect(shouldSearch('a')).toBe(false)
    expect(shouldSearch('ab')).toBe(true)
    expect(shouldSearch('abc')).toBe(true)
  })
})
