import { describe, expect, it } from 'vitest'
import { filterTags } from './TagCloudPage'
import type { Tag } from '../types'

function makeTag(value: string): Tag {
  return { id: value, key: 'genre', value, scope: 'metadata' }
}

const tags = [
  makeTag('Action'),
  makeTag('Comedy'),
  makeTag('action thriller'),
  makeTag('Drama'),
]

describe('filterTags', () => {
  it('returns all tags when search is empty', () => {
    expect(filterTags(tags, '')).toEqual(tags)
  })

  it('filters case-insensitively', () => {
    expect(filterTags(tags, 'action')).toEqual([makeTag('Action'), makeTag('action thriller')])
    expect(filterTags(tags, 'ACTION')).toEqual([makeTag('Action'), makeTag('action thriller')])
  })

  it('returns empty array when nothing matches', () => {
    expect(filterTags(tags, 'horror')).toEqual([])
  })

  it('matches partial substrings', () => {
    expect(filterTags(tags, 'edy')).toEqual([makeTag('Comedy')])
  })

  it('returns empty array for empty tag list', () => {
    expect(filterTags([], 'action')).toEqual([])
  })
})
