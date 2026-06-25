import { describe, it, expect } from 'vitest'
import { MODULE_REGISTRY } from './modules'
import type { EnabledModules } from '../context/ModulesContext'

describe('MODULE_REGISTRY', () => {
  it('contains no duplicate keys', () => {
    const keys = MODULE_REGISTRY.map(m => m.key)
    expect(new Set(keys).size).toBe(keys.length)
  })

  it('contains no duplicate paths', () => {
    const paths = MODULE_REGISTRY.map(m => m.path)
    expect(new Set(paths).size).toBe(paths.length)
  })

  it('has an entry for every EnabledModules key', () => {
    const allKeys: (keyof EnabledModules)[] = ['movies', 'tv', 'music', 'books', 'afterdark', 'jav']
    const registryKeys = MODULE_REGISTRY.map(m => m.key)
    for (const key of allKeys) {
      expect(registryKeys).toContain(key)
    }
  })

  it('every entry has a non-empty label, path starting with /, icon, and hex accent', () => {
    for (const entry of MODULE_REGISTRY) {
      expect(entry.label.length).toBeGreaterThan(0)
      expect(entry.path.startsWith('/')).toBe(true)
      expect(entry.icon).toBeDefined()
      expect(entry.accent.startsWith('#')).toBe(true)
    }
  })

  it('includes jav entry', () => {
    expect(MODULE_REGISTRY.find(m => m.key === 'jav')).toBeDefined()
  })
})
