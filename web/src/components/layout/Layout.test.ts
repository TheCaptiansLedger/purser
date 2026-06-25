import { describe, expect, it } from 'vitest'
import { parseSidebarCollapsed } from './Layout'

describe('parseSidebarCollapsed', () => {
  it('returns true for the exact string "true"', () => {
    expect(parseSidebarCollapsed('true')).toBe(true)
  })

  it('returns false for null (nothing stored yet)', () => {
    expect(parseSidebarCollapsed(null)).toBe(false)
  })

  it('returns false for "false"', () => {
    expect(parseSidebarCollapsed('false')).toBe(false)
  })

  it('returns false for empty string', () => {
    expect(parseSidebarCollapsed('')).toBe(false)
  })

  it('returns false for "True" (case-sensitive)', () => {
    expect(parseSidebarCollapsed('True')).toBe(false)
  })
})
