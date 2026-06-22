import { describe, expect, it } from 'vitest'
import { setsEqual } from './useEditForm'

describe('setsEqual', () => {
  it('returns true for two empty sets', () => {
    expect(setsEqual(new Set(), new Set())).toBe(true)
  })

  it('returns true for equal sets', () => {
    expect(setsEqual(new Set(['name', 'overview']), new Set(['name', 'overview']))).toBe(true)
  })

  it('returns true regardless of insertion order', () => {
    expect(setsEqual(new Set(['overview', 'name']), new Set(['name', 'overview']))).toBe(true)
  })

  it('returns false when sizes differ', () => {
    expect(setsEqual(new Set(['name']), new Set(['name', 'overview']))).toBe(false)
  })

  it('returns false when values differ but size is the same', () => {
    expect(setsEqual(new Set(['name']), new Set(['overview']))).toBe(false)
  })

  it('returns false when one set is empty and the other is not', () => {
    expect(setsEqual(new Set(), new Set(['name']))).toBe(false)
    expect(setsEqual(new Set(['name']), new Set())).toBe(false)
  })
})
