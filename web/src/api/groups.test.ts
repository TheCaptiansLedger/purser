import { describe, expect, it } from 'vitest'
import { sortGroupsByYear } from './groups'
import type { Group } from '../types'

function makeGroup(id: string, year: number): Group {
  return {
    id,
    libraryEntryId: 'entry-1',
    title: `Album ${id}`,
    sortName: `Album ${id}`,
    number: 0,
    year,
    overview: '',
    monitored: false,
    monitorMode: 'all',
  }
}

const a2000 = makeGroup('a', 2000)
const b1990 = makeGroup('b', 1990)
const c2010 = makeGroup('c', 2010)
const d0    = makeGroup('d', 0)
const e0    = makeGroup('e', 0)

describe('sortGroupsByYear', () => {
  it('sorts oldest first with asc', () => {
    const result = sortGroupsByYear([c2010, a2000, b1990], 'asc')
    expect(result.map(g => g.year)).toEqual([1990, 2000, 2010])
  })

  it('sorts newest first with desc', () => {
    const result = sortGroupsByYear([b1990, c2010, a2000], 'desc')
    expect(result.map(g => g.year)).toEqual([2010, 2000, 1990])
  })

  it('places year=0 last in asc order', () => {
    const result = sortGroupsByYear([d0, b1990, c2010], 'asc')
    expect(result[result.length - 1].id).toBe('d')
  })

  it('places year=0 last in desc order', () => {
    const result = sortGroupsByYear([d0, b1990, c2010], 'desc')
    expect(result[result.length - 1].id).toBe('d')
  })

  it('handles multiple year=0 entries without error', () => {
    const result = sortGroupsByYear([d0, b1990, e0], 'asc')
    expect(result.map(g => g.year)).toEqual([1990, 0, 0])
  })

  it('does not mutate the original array', () => {
    const original = [c2010, b1990, a2000]
    sortGroupsByYear(original, 'asc')
    expect(original.map(g => g.id)).toEqual(['c', 'b', 'a'])
  })

  it('returns an empty array unchanged', () => {
    expect(sortGroupsByYear([], 'asc')).toEqual([])
  })

  it('returns a single-item array unchanged', () => {
    expect(sortGroupsByYear([a2000], 'desc')).toEqual([a2000])
  })
})
