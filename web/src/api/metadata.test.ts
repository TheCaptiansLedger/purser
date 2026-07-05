import { describe, expect, it } from 'vitest'
import { entryKindForContentType } from './metadata'
import type { ContentType } from '../types'

describe('entryKindForContentType', () => {
  const cases: [ContentType, string][] = [
    ['music',  'artist'],
    ['adult',  'studio'],
    ['jav',    'studio'],
    ['tv',     'series'],
    ['movie',  'movie'],
    ['book',   'author'],
  ]

  it.each(cases)('%s → %s', (ct, expected) => {
    expect(entryKindForContentType(ct)).toBe(expected)
  })
})
