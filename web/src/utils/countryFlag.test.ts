import { describe, expect, it } from 'vitest'
import { countryFlag, countryName, isKnownCountry } from './countryFlag'

describe('countryFlag', () => {
  it('converts ISO alpha-2 codes to regional indicator flags', () => {
    expect(countryFlag('US')).toBe(String.fromCodePoint(0x1f1fa, 0x1f1f8))
    expect(countryFlag('it')).toBe(String.fromCodePoint(0x1f1ee, 0x1f1f9))
  })

  it('returns an empty string for non alpha-2 values', () => {
    expect(countryFlag('USA')).toBe('')
    expect(countryFlag('')).toBe('')
  })
})

describe('countryName', () => {
  it('returns readable names for bundled countries', () => {
    expect(countryName('US')).toBe('United States')
    expect(countryName('gb')).toBe('United Kingdom')
  })

  it('returns the raw trimmed code for unknown countries', () => {
    expect(countryName('XX')).toBe('XX')
    expect(countryName(' Atlantis ')).toBe('Atlantis')
  })
})

describe('isKnownCountry', () => {
  it('distinguishes bundled countries from unknown values', () => {
    expect(isKnownCountry('US')).toBe(true)
    expect(isKnownCountry('XX')).toBe(false)
  })
})
