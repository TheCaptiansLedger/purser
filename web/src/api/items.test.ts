import { describe, expect, it } from 'vitest'
import { SORT_OPTIONS, SORT_DIR_OPTIONS } from './items'

describe('SORT_OPTIONS', () => {
  it('includes date and title', () => {
    expect(SORT_OPTIONS).toContain('date')
    expect(SORT_OPTIONS).toContain('title')
  })

  it('has exactly two options', () => {
    expect(SORT_OPTIONS).toHaveLength(2)
  })
})

describe('SORT_DIR_OPTIONS', () => {
  it('includes asc and desc', () => {
    expect(SORT_DIR_OPTIONS).toContain('asc')
    expect(SORT_DIR_OPTIONS).toContain('desc')
  })

  it('has exactly two options', () => {
    expect(SORT_DIR_OPTIONS).toHaveLength(2)
  })
})
