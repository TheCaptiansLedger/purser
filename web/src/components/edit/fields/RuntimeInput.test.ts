import { describe, expect, it } from 'vitest'
import { secondsToHMS, hmsToSeconds } from './RuntimeInput'

describe('secondsToHMS', () => {
  it('converts zero', () => expect(secondsToHMS(0)).toEqual({ h: 0, m: 0, s: 0 }))
  it('converts whole hours', () => expect(secondsToHMS(7200)).toEqual({ h: 2, m: 0, s: 0 }))
  it('converts mixed h/m/s', () => expect(secondsToHMS(3723)).toEqual({ h: 1, m: 2, s: 3 }))
  it('does not carry seconds into minutes when s < 60', () => expect(secondsToHMS(59)).toEqual({ h: 0, m: 0, s: 59 }))
})

describe('hmsToSeconds', () => {
  it('converts zero', () => expect(hmsToSeconds(0, 0, 0)).toBe(0))
  it('converts hours only', () => expect(hmsToSeconds(2, 0, 0)).toBe(7200))
  it('converts mixed', () => expect(hmsToSeconds(1, 2, 3)).toBe(3723))
  it('round-trips secondsToHMS', () => {
    const { h, m, s } = secondsToHMS(5432)
    expect(hmsToSeconds(h, m, s)).toBe(5432)
  })
})
