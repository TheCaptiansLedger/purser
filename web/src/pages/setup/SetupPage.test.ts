import { describe, expect, it } from 'vitest'
import { DEFAULT_MODULES, MODULE_META } from './SetupPage'
import type { ModuleKey } from './SetupPage'

const MODULE_KEYS: ModuleKey[] = ['movies', 'tv', 'music', 'afterdark', 'books']

describe('DEFAULT_MODULES', () => {
  it('enables movies, tv, and music by default', () => {
    expect(DEFAULT_MODULES.movies).toBe(true)
    expect(DEFAULT_MODULES.tv).toBe(true)
    expect(DEFAULT_MODULES.music).toBe(true)
  })

  it('disables afterdark and books by default', () => {
    expect(DEFAULT_MODULES.afterdark).toBe(false)
    expect(DEFAULT_MODULES.books).toBe(false)
  })
})

describe('MODULE_META', () => {
  it('has an entry for every module key', () => {
    for (const key of MODULE_KEYS) {
      expect(MODULE_META[key]).toBeDefined()
    }
  })

  it('every entry has a non-empty label, description, and icon', () => {
    for (const key of MODULE_KEYS) {
      const { label, description, icon } = MODULE_META[key]
      expect(label.length).toBeGreaterThan(0)
      expect(description.length).toBeGreaterThan(0)
      expect(icon.length).toBeGreaterThan(0)
    }
  })
})
