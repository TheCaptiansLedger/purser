import { describe, expect, it } from 'vitest'
import { hitRate } from './cacheFormat'

describe('hitRate', () => {
  it('rounds hits over total to a whole-number percentage', () => {
    expect(hitRate(80, 20)).toBe(80)
  })

  it('rounds to the nearest whole number', () => {
    expect(hitRate(1, 2)).toBe(33)
  })

  it('returns 0 when there have been no requests yet', () => {
    expect(hitRate(0, 0)).toBe(0)
  })

  it('returns 100 when every request was a hit', () => {
    expect(hitRate(5, 0)).toBe(100)
  })
})
