import { describe, expect, it } from 'vitest'
import { canAddExternalID } from './ExternalIDList'

describe('canAddExternalID', () => {
  it('returns true when both source and value are non-empty', () => {
    expect(canAddExternalID('tmdb', '12345')).toBe(true)
    expect(canAddExternalID('stashdb', 'abc-def')).toBe(true)
  })

  it('returns false when source is empty', () => {
    expect(canAddExternalID('', '12345')).toBe(false)
    expect(canAddExternalID('   ', '12345')).toBe(false)
  })

  it('returns false when value is empty', () => {
    expect(canAddExternalID('tmdb', '')).toBe(false)
    expect(canAddExternalID('tmdb', '   ')).toBe(false)
  })

  it('returns false when both are empty', () => {
    expect(canAddExternalID('', '')).toBe(false)
  })

  it('trims whitespace before checking', () => {
    expect(canAddExternalID('  tmdb  ', '  12345  ')).toBe(true)
  })
})
