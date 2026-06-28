import { describe, expect, it } from 'vitest'
import { shouldCloseLightbox } from './Lightbox'

describe('shouldCloseLightbox', () => {
  it('returns true for Escape', () => {
    expect(shouldCloseLightbox('Escape')).toBe(true)
  })
  it('returns false for other keys', () => {
    expect(shouldCloseLightbox('ArrowLeft')).toBe(false)
    expect(shouldCloseLightbox('Enter')).toBe(false)
    expect(shouldCloseLightbox('')).toBe(false)
  })
})
