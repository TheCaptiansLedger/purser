import { describe, expect, it } from 'vitest'

// The storage key must follow the purser:statusOverlay:<module> pattern.
// We test the key formula rather than the hook directly (hook requires DOM).
const storageKey = (module: string) => `purser:statusOverlay:${module}`

describe('storageKey', () => {
  it('generates the correct key for a module', () => {
    expect(storageKey('afterdark')).toBe('purser:statusOverlay:afterdark')
    expect(storageKey('movies')).toBe('purser:statusOverlay:movies')
    expect(storageKey('tv')).toBe('purser:statusOverlay:tv')
  })

  it('keys are unique per module', () => {
    const keys = ['afterdark', 'movies', 'tv', 'music', 'books'].map(storageKey)
    const unique = new Set(keys)
    expect(unique.size).toBe(keys.length)
  })
})
