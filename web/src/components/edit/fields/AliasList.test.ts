import { describe, expect, it } from 'vitest'
import { canAddAlias } from './AliasList'

describe('canAddAlias', () => {
  it('returns true for a non-empty string not in the list', () => {
    expect(canAddAlias('AKA Name', [])).toBe(true)
    expect(canAddAlias('AKA Name', ['Other Name'])).toBe(true)
  })

  it('returns false for an empty string', () => {
    expect(canAddAlias('', [])).toBe(false)
    expect(canAddAlias('   ', [])).toBe(false)
  })

  it('returns false when the alias already exists (exact match)', () => {
    expect(canAddAlias('AKA Name', ['AKA Name'])).toBe(false)
  })

  it('returns false when the alias already exists after trimming', () => {
    expect(canAddAlias('  AKA Name  ', ['AKA Name'])).toBe(false)
  })

  it('is case-sensitive', () => {
    expect(canAddAlias('aka name', ['AKA Name'])).toBe(true)
  })
})
