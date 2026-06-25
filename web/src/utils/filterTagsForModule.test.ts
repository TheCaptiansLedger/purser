import { describe, it, expect } from 'vitest'
import { filterTagsForModule } from './filterTagsForModule'
import type { Tag } from '../types'

const make = (key: string, value: string): Tag => ({ id: key + value, key, value, scope: 'metadata' })

const tags: Tag[] = [
  make('genre', 'Drama'),
  make('adult', 'explicit'),
  make('label', 'Atlantic Records'),
]

describe('filterTagsForModule', () => {
  it('returns all tags for adult content type', () => {
    expect(filterTagsForModule(tags, 'adult')).toEqual(tags)
  })

  it('strips adult tags for non-afterdark modules', () => {
    const result = filterTagsForModule(tags, 'music')
    expect(result).toHaveLength(2)
    expect(result.every(t => t.key !== 'adult')).toBe(true)
  })

  it('strips adult tags for movie content type', () => {
    const result = filterTagsForModule(tags, 'movie')
    expect(result.find(t => t.key === 'adult')).toBeUndefined()
  })

  it('returns empty array when all tags are adult and module is not afterdark', () => {
    const adultOnly = [make('adult', 'test')]
    expect(filterTagsForModule(adultOnly, 'tv')).toHaveLength(0)
  })

  it('returns all tags when there are no adult tags', () => {
    const noAdult = [make('genre', 'Comedy'), make('label', 'Sony')]
    expect(filterTagsForModule(noAdult, 'music')).toEqual(noAdult)
  })
})
