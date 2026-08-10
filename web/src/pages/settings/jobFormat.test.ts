import { describe, expect, it } from 'vitest'
import { formatJobProgress, formatJobTimestamp } from './jobFormat'

describe('formatJobTimestamp', () => {
  it('renders an em dash for an unset timestamp', () => {
    expect(formatJobTimestamp(undefined)).toBe('—')
  })

  it('renders a set timestamp via toLocaleString', () => {
    const date = new Date('2026-08-09T12:00:00Z')
    expect(formatJobTimestamp(date)).toBe(date.toLocaleString())
  })
})

describe('formatJobProgress', () => {
  it('renders a fraction as a rounded percentage', () => {
    expect(formatJobProgress(0.4)).toBe('40%')
  })

  it('renders a complete job as 100%', () => {
    expect(formatJobProgress(1)).toBe('100%')
  })

  it('rounds a fraction that does not divide evenly', () => {
    expect(formatJobProgress(1 / 3)).toBe('33%')
  })
})
