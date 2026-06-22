import { describe, expect, it } from 'vitest'
import {
  DEFAULT_MODULES,
  DEFAULT_ROOTS,
  MODULE_META,
  SOURCE_DEFS,
  SOURCE_CONFIG_PREFIX,
  MODULE_SOURCES,
  sourcesForModules,
  canProceedFromSources,
  canProceedFromRoots,
  makeInitialSourceState,
  configKeyToEnvVar,
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

describe('configKeyToEnvVar', () => {
  it('converts a dotted key to PURSER_ env var format', () => {
    expect(configKeyToEnvVar('sources.tmdb.api_key')).toBe('PURSER_SOURCES_TMDB_API_KEY')
  })

  it('handles module enabled keys', () => {
    expect(configKeyToEnvVar('modules.movies.enabled')).toBe('PURSER_MODULES_MOVIES_ENABLED')
  })

  it('handles module roots keys', () => {
    expect(configKeyToEnvVar('modules.afterdark.roots')).toBe('PURSER_MODULES_AFTERDARK_ROOTS')
  })

  it('maps every source API key to the correct env var', () => {
    expect(configKeyToEnvVar(`${SOURCE_CONFIG_PREFIX.tmdb}.api_key`)).toBe('PURSER_SOURCES_TMDB_API_KEY')
    expect(configKeyToEnvVar(`${SOURCE_CONFIG_PREFIX.tvdb}.api_key`)).toBe('PURSER_SOURCES_TVDB_API_KEY')
    expect(configKeyToEnvVar(`${SOURCE_CONFIG_PREFIX.mbz}.api_key`)).toBe('PURSER_SOURCES_MUSICBRAINZ_API_KEY')
    expect(configKeyToEnvVar(`${SOURCE_CONFIG_PREFIX.audiodb}.api_key`)).toBe('PURSER_SOURCES_THEAUDIODB_API_KEY')
    expect(configKeyToEnvVar(`${SOURCE_CONFIG_PREFIX.fanart}.api_key`)).toBe('PURSER_SOURCES_FANART_API_KEY')
    expect(configKeyToEnvVar(`${SOURCE_CONFIG_PREFIX.stashdb}.api_key`)).toBe('PURSER_SOURCES_STASHDB_API_KEY')
  })
})

describe('SOURCE_CONFIG_PREFIX', () => {
  it('has an entry for every source ID', () => {
    for (const id of ALL_SOURCE_IDS) {
      expect(SOURCE_CONFIG_PREFIX[id]).toBeDefined()
    }
  })

  it('all prefixes start with sources.', () => {
    for (const id of ALL_SOURCE_IDS) {
      expect(SOURCE_CONFIG_PREFIX[id].startsWith('sources.')).toBe(true)
    }
  })

  it('maps mbz to sources.musicbrainz', () => {
    expect(SOURCE_CONFIG_PREFIX.mbz).toBe('sources.musicbrainz')
  })

  it('maps audiodb to sources.theaudiodb', () => {
    expect(SOURCE_CONFIG_PREFIX.audiodb).toBe('sources.theaudiodb')
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

describe('makeInitialSourceState', () => {
  it('initializes unlocked sources as idle with empty apiKey', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    const states = makeInitialSourceState(defs)
    expect(states.tmdb.apiKey).toBe('')
    expect(states.tmdb.status).toBe('idle')
    expect(states.tmdb.locked).toBe(false)
  })

  it('initializes locked sources with masked apiKey and idle status', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    const states = makeInitialSourceState(defs, { 'sources.tmdb.api_key': true })
    expect(states.tmdb.apiKey).toBe('***')
    expect(states.tmdb.status).toBe('idle')
    expect(states.tmdb.locked).toBe(true)
  })

  it('does not lock mbz even if its config key is present — mbz has no api key', () => {
    const defs = sourcesForModules({ ...NO_MODULES, music: true })
    const states = makeInitialSourceState(defs, { 'sources.musicbrainz.api_key': true })
    expect(states.mbz.locked).toBe(false)
  })
})

describe('DEFAULT_ROOTS', () => {
  it('has an entry for every module key', () => {
    for (const key of MODULE_KEYS) {
      expect(DEFAULT_ROOTS[key]).toBeDefined()
    }
  })

  it('every default entry is a non-empty array', () => {
    for (const key of MODULE_KEYS) {
      expect(DEFAULT_ROOTS[key].length).toBeGreaterThan(0)
    }
  })

  it('every default path starts with /', () => {
    for (const key of MODULE_KEYS) {
      for (const path of DEFAULT_ROOTS[key]) {
        expect(path.startsWith('/')).toBe(true)
      }
    }
  })

  it('allows canProceedFromRoots to pass with all modules enabled', () => {
    const allEnabled: ModuleState = {
      movies: true, tv: true, music: true, afterdark: true, books: true,
    }
    expect(canProceedFromRoots(allEnabled, DEFAULT_ROOTS)).toBe(true)
  })
})

describe('canProceedFromRoots', () => {
  const validRoots: Record<ModuleKey, string[]> = {
    movies: ['/media/movies'], tv: ['/media/tv'], music: ['/media/music'],
    afterdark: ['/media/afterdark'], books: ['/media/books'],
  }
  const emptyRoots: Record<ModuleKey, string[]> = {
    movies: [''], tv: [''], music: [''], afterdark: [''], books: [''],
  }

  it('returns false when any enabled module has an empty path', () => {
    expect(canProceedFromRoots({ ...NO_MODULES, movies: true }, emptyRoots)).toBe(false)
  })

  it('returns false when a path does not start with /', () => {
    expect(canProceedFromRoots(
      { ...NO_MODULES, movies: true },
      { ...emptyRoots, movies: ['media/movies'] },
    )).toBe(false)
  })

  it('returns false when an empty path is mixed in with valid paths', () => {
    expect(canProceedFromRoots(
      { ...NO_MODULES, movies: true },
      { ...validRoots, movies: ['/media/movies', ''] },
    )).toBe(false)
  })

  it('returns false when a module has no paths at all', () => {
    expect(canProceedFromRoots(
      { ...NO_MODULES, movies: true },
      { ...validRoots, movies: [] },
    )).toBe(false)
  })

  it('returns true when all enabled modules have valid paths', () => {
    expect(canProceedFromRoots({ ...NO_MODULES, movies: true }, validRoots)).toBe(true)
  })

  it('returns true when a module has multiple valid paths', () => {
    expect(canProceedFromRoots(
      { ...NO_MODULES, movies: true },
      { ...validRoots, movies: ['/media/movies', '/mnt/nas/movies'] },
    )).toBe(true)
  })

  it('returns true when multiple modules all have valid paths', () => {
    expect(canProceedFromRoots({ ...NO_MODULES, movies: true, tv: true }, validRoots)).toBe(true)
  })

  it('ignores disabled modules — their paths are irrelevant', () => {
    expect(canProceedFromRoots({ ...NO_MODULES, movies: true }, { ...emptyRoots, movies: ['/media/movies'] })).toBe(true)
  })

  it('returns true when no modules are enabled', () => {
    expect(canProceedFromRoots(NO_MODULES, emptyRoots)).toBe(true)
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

  it('returns true for a locked source even when status is idle and not skipped', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    expect(canProceedFromSources(defs, { tmdb: idle }, {}, { tmdb: true })).toBe(true)
  })

  it('returns true when one source is locked and another is verified ok', () => {
    const defs = sourcesForModules({ ...NO_MODULES, tv: true })
    expect(canProceedFromSources(defs, { tvdb: idle, tmdb: ok }, {}, { tvdb: true })).toBe(true)
  })

  it('backward compatible — no locked arg behaves as before', () => {
    const defs = sourcesForModules({ ...NO_MODULES, movies: true })
    expect(canProceedFromSources(defs, { tmdb: idle }, {})).toBe(false)
    expect(canProceedFromSources(defs, { tmdb: ok }, {})).toBe(true)
  })
})
