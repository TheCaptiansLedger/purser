import { describe, expect, it } from 'vitest'
import { toSlug, slugToLabel } from './toSlug'

describe('toSlug', () => {
  it('lowercases the input', () => {
    expect(toSlug('Action')).toBe('action')
  })

  it('replaces spaces with hyphens', () => {
    expect(toSlug('Science Fiction')).toBe('science-fiction')
  })

  it('collapses multiple spaces into a single hyphen', () => {
    expect(toSlug('Rock  Roll')).toBe('rock-roll')
  })

  it('preserves existing hyphens', () => {
    expect(toSlug('Sci-Fi')).toBe('sci-fi')
  })

  it('strips non-alphanumeric non-hyphen characters', () => {
    expect(toSlug('Crime & Thriller')).toBe('crime--thriller')
  })

  it('handles empty string', () => {
    expect(toSlug('')).toBe('')
  })
})

describe('slugToLabel', () => {
  it('replaces hyphens with spaces', () => {
    expect(slugToLabel('science-fiction')).toBe('Science Fiction')
  })

  it('title-cases each word', () => {
    expect(slugToLabel('action')).toBe('Action')
  })

  it('handles multi-word slugs', () => {
    expect(slugToLabel('crime-and-thriller')).toBe('Crime And Thriller')
  })

  it('handles empty string', () => {
    expect(slugToLabel('')).toBe('')
  })
})
