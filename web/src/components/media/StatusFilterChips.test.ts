import { describe, expect, it } from 'vitest'
import { STATUSES } from './StatusFilterChips'

describe('STATUSES', () => {
  it('covers every known ItemStatus value', () => {
    expect(STATUSES).toEqual(['wanted', 'grabbed', 'downloading', 'imported', 'missing', 'skipped'])
  })

  it('contains no duplicate values', () => {
    expect(new Set(STATUSES).size).toBe(STATUSES.length)
  })
})
