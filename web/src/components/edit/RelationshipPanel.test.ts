import { describe, expect, it } from 'vitest'
import { itemPersonRoles, entryPersonRoles } from './RelationshipPanel'
import type { ContentTypeConfig, KindConfig } from '../../types'

const contentTypeConfigs: ContentTypeConfig[] = [
  { contentType: 'adult', personRoles: ['performer', 'actress', 'actor', 'director'] },
  { contentType: 'jav',   personRoles: ['performer', 'actress', 'actor', 'director'] },
  { contentType: 'tv',    personRoles: ['actor', 'actress', 'director', 'guest_star', 'writer'] },
  { contentType: 'movie', personRoles: ['actor', 'actress', 'director', 'producer', 'writer'] },
  { contentType: 'music', personRoles: ['artist', 'featured_artist', 'producer', 'songwriter'] },
  { contentType: 'book',  personRoles: ['author', 'editor', 'illustrator', 'narrator'] },
]

const kindConfigs: KindConfig[] = [
  { kind: 'artist',    personRoles: ['member', 'former_member', 'vocalist', 'guitarist', 'bassist', 'drummer', 'keyboardist', 'producer'], showDates: true },
  { kind: 'studio',    personRoles: ['performer', 'director', 'contracted_performer'], showDates: false },
  { kind: 'network',   personRoles: ['affiliated_performer', 'director', 'producer'], showDates: false },
  { kind: 'series',    personRoles: ['regular_cast', 'recurring_cast', 'director', 'producer', 'writer'], showDates: false },
  { kind: 'movie',     personRoles: ['actor', 'actress', 'director', 'producer', 'writer'], showDates: false },
  { kind: 'book',      personRoles: ['author', 'editor', 'narrator', 'illustrator'], showDates: false },
  { kind: 'publisher', personRoles: ['author', 'editor'], showDates: false },
]

describe('itemPersonRoles', () => {
  it('returns performer roles for adult content', () => {
    const roles = itemPersonRoles(contentTypeConfigs, 'adult')
    expect(roles).toContain('performer')
    expect(roles).toContain('director')
  })

  it('returns performer roles for jav content', () => {
    const roles = itemPersonRoles(contentTypeConfigs, 'jav')
    expect(roles).toContain('performer')
  })

  it('returns cast roles for tv content', () => {
    const roles = itemPersonRoles(contentTypeConfigs, 'tv')
    expect(roles).toContain('actor')
    expect(roles).toContain('director')
    expect(roles).toContain('guest_star')
  })

  it('returns actor/director roles for movie content', () => {
    const roles = itemPersonRoles(contentTypeConfigs, 'movie')
    expect(roles).toContain('actor')
    expect(roles).toContain('actress')
    expect(roles).toContain('director')
  })

  it('returns artist roles for music items', () => {
    const roles = itemPersonRoles(contentTypeConfigs, 'music')
    expect(roles).toContain('artist')
    expect(roles).toContain('featured_artist')
  })

  it('returns author roles for book items', () => {
    const roles = itemPersonRoles(contentTypeConfigs, 'book')
    expect(roles).toContain('author')
    expect(roles).toContain('narrator')
  })

  it('falls back to performer when config is empty', () => {
    const roles = itemPersonRoles([], 'adult')
    expect(roles).toEqual(['performer'])
  })

  it('falls back to performer for unknown content type', () => {
    const roles = itemPersonRoles(contentTypeConfigs, 'unknown')
    expect(roles).toEqual(['performer'])
  })
})

describe('entryPersonRoles', () => {
  it('returns band-member roles for artist kind', () => {
    const roles = entryPersonRoles(kindConfigs, 'artist')
    expect(roles).toContain('member')
    expect(roles).toContain('vocalist')
    expect(roles).toContain('former_member')
  })

  it('returns performer roles for studio kind', () => {
    const roles = entryPersonRoles(kindConfigs, 'studio')
    expect(roles).toContain('performer')
    expect(roles).toContain('director')
  })

  it('returns cast roles for series kind', () => {
    const roles = entryPersonRoles(kindConfigs, 'series')
    expect(roles).toContain('regular_cast')
    expect(roles).toContain('director')
  })

  it('returns author roles for publisher kind', () => {
    const roles = entryPersonRoles(kindConfigs, 'publisher')
    expect(roles).toContain('author')
    expect(roles).toContain('editor')
  })

  it('falls back to member when config is empty', () => {
    const roles = entryPersonRoles([], 'artist')
    expect(roles).toEqual(['member'])
  })

  it('falls back to member for undefined kind', () => {
    const roles = entryPersonRoles(kindConfigs, undefined)
    expect(roles).toEqual(['member'])
  })

  it('falls back to member for unknown kind', () => {
    const roles = entryPersonRoles(kindConfigs, 'unknown')
    expect(roles).toEqual(['member'])
  })
})
