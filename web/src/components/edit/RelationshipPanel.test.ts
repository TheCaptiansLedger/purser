import { describe, expect, it } from 'vitest'
import { rolesFor } from './RelationshipPanel'

describe('rolesFor — entry', () => {
  it('returns band-member roles for artist kind', () => {
    const roles = rolesFor('entry', 'music', 'artist')
    expect(roles).toContain('member')
    expect(roles).toContain('vocalist')
    expect(roles).toContain('former_member')
  })

  it('returns performer roles for studio kind', () => {
    const roles = rolesFor('entry', 'adult', 'studio')
    expect(roles).toContain('performer')
    expect(roles).toContain('director')
  })

  it('returns cast roles for series kind', () => {
    const roles = rolesFor('entry', 'tv', 'series')
    expect(roles).toContain('regular_cast')
    expect(roles).toContain('director')
  })

  it('returns author roles for publisher kind', () => {
    const roles = rolesFor('entry', 'book', 'publisher')
    expect(roles).toContain('author')
    expect(roles).toContain('editor')
  })

  it('falls back to member for unknown kind', () => {
    const roles = rolesFor('entry', 'movie')
    expect(roles).toEqual(['member'])
  })
})

describe('rolesFor — item', () => {
  it('returns performer roles for adult content', () => {
    const roles = rolesFor('item', 'adult')
    expect(roles).toContain('performer')
    expect(roles).toContain('director')
  })

  it('returns actor/director roles for movie content', () => {
    const roles = rolesFor('item', 'movie')
    expect(roles).toContain('actor')
    expect(roles).toContain('actress')
    expect(roles).toContain('director')
  })

  it('returns author roles for book items', () => {
    const roles = rolesFor('item', 'book')
    expect(roles).toContain('author')
    expect(roles).toContain('narrator')
  })

  it('returns artist roles for music items', () => {
    const roles = rolesFor('item', 'music')
    expect(roles).toContain('artist')
    expect(roles).toContain('featured_artist')
  })

  it('treats jav the same as adult', () => {
    const roles = rolesFor('item', 'jav')
    expect(roles).toContain('performer')
  })
})
