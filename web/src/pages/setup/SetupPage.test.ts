import { describe, expect, it } from 'vitest'
import {
  DEFAULT_MODULES,
  MODULE_META,
  SOURCE_DEFS,
  MODULE_SOURCES,
  sourcesForModules,
  canProceedFromSources,
} from './SetupPage'
import type { ModuleKey, ModuleState, SourceID } from './SetupPage'

const MODULE_KEYS: ModuleKey[] = ['movies', 'tv', 'music', 'afterdark', 'books']
const ALL_SOURCE_IDS: SourceID[] = ['tmdb', 'tvdb', 'mbz', 'audiodb', 'fanart', 'stashdb']

const NO_MODULES: ModuleState = {
  movies: false, tv: false, music: false, afterdark: false, books: false,
}

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

describe('SOURCE_DEFS', () => {
  it('has an entry for every source ID', () => {
    for (const id of ALL_SOURCE_IDS) {
      expect(SOURCE_DEFS[id]).toBeDefined()
    }
  })

  it('mbz does not require an API key', () => {
    expect(SOURCE_DEFS.mbz.requiresApiKey).toBe(false)
  })

  it('stashdb has an endpoint URL field', () => {
    expect(SOURCE_DEFS.stashdb.hasEndpointUrl).toBe(true)
  })

  it('only stashdb has an endpoint URL field', () => {
    for (const id of ALL_SOURCE_IDS.filter((id) => id !== 'stashdb')) {
      expect(SOURCE_DEFS[id].hasEndpointUrl).toBe(false)
    }
  })
})

describe('MODULE_SOURCES', () => {
  it('movies includes tmdb', () => {
    expect(MODULE_SOURCES.movies).toContain('tmdb')
  })

  it('tv includes both tvdb and tmdb', () => {
    expect(MODULE_SOURCES.tv).toContain('tvdb')
    expect(MODULE_SOURCES.tv).toContain('tmdb')
  })

  it('music includes mbz, audiodb, and fanart', () => {
    expect(MODULE_SOURCES.music).toContain('mbz')
    expect(MODULE_SOURCES.music).toContain('audiodb')
    expect(MODULE_SOURCES.music).toContain('fanart')
  })

  it('afterdark includes stashdb', () => {
    expect(MODULE_SOURCES.afterdark).toContain('stashdb')
  })

  it('books is empty', () => {
    expect(MODULE_SOURCES.books).toHaveLength(0)
  })
})

describe('sourcesForModules', () => {
  it('returns empty array when no modules are enabled', () => {
    expect(sourcesForModules(NO_MODULES)).toHaveLength(0)
  })

  it('returns tmdb for movies-only', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    expect(defs.map((d) => d.id)).toEqual(['tmdb'])
  })

  it('deduplicates tmdb when both movies and tv are enabled', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true, tv: true })
    const ids = defs.map((d) => d.id)
    expect(ids.filter((id) => id === 'tmdb')).toHaveLength(1)
  })

  it('includes tvdb and tmdb (deduplicated) for tv + movies', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true, tv: true })
    const ids = defs.map((d) => d.id)
    expect(ids).toContain('tvdb')
    expect(ids).toContain('tmdb')
    expect(ids).toHaveLength(2)
  })

  it('returns mbz, audiodb, and fanart for music', () => {
    const defs = sourcesForModules({ ...NO_MODULES, music: true })
    const ids = defs.map((d) => d.id)
    expect(ids).toContain('mbz')
    expect(ids).toContain('audiodb')
    expect(ids).toContain('fanart')
  })

  it('returns stashdb for afterdark', () => {
    const defs = sourcesForModules({ ...NO_MODULES, afterdark: true })
    expect(defs.map((d) => d.id)).toEqual(['stashdb'])
  })

  it('excludes books (no sources defined)', () => {
    const defs = sourcesForModules({ ...NO_MODULES, books: true })
    expect(defs).toHaveLength(0)
  })
})

describe('canProceedFromSources', () => {
  const idle = 'idle' as const
  const ok   = 'ok' as const

  it('mbz does not block proceed — only audiodb and fanart do for music', () => {
    const defs = sourcesForModules({ ...NO_MODULES, music: true })
    const allIdle: Record<string, 'idle' | 'loading' | 'ok' | 'error'> = {
      mbz: idle, audiodb: idle, fanart: idle,
    }
    expect(canProceedFromSources(defs, allIdle, {})).toBe(false)
    expect(canProceedFromSources(defs, { mbz: idle, audiodb: ok, fanart: ok }, {})).toBe(true)
    expect(canProceedFromSources(defs, allIdle, { audiodb: true, fanart: true })).toBe(true)
  })

  it('returns false when a required source is idle and not skipped', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    expect(canProceedFromSources(defs, { tmdb: idle }, {})).toBe(false)
  })

  it('returns true when all required sources are verified ok', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    expect(canProceedFromSources(defs, { tmdb: ok }, {})).toBe(true)
  })

  it('returns true when all required sources are skipped', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    expect(canProceedFromSources(defs, { tmdb: idle }, { tmdb: true })).toBe(true)
  })

  it('returns false when one of several sources is neither ok nor skipped', () => {
    const defs = sourcesForModules({ ...NO_MODULES, tv: true })
    expect(canProceedFromSources(defs, { tvdb: ok, tmdb: idle }, {})).toBe(false)
  })
})
