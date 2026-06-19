import { describe, expect, it } from 'vitest'
import { artistImportRequest } from './MusicPage'
import type { ExternalStudio } from '../../types'

const base: ExternalStudio = {
  source: 'mbz',
  externalId: 'a74b1b7f-71a5-4011-9441-d0b5e4122711',
  name: 'Radiohead',
}

describe('artistImportRequest', () => {
  it('maps source and externalId from candidate', () => {
    const req = artistImportRequest(base)
    expect(req.source).toBe('mbz')
    expect(req.externalId).toBe('a74b1b7f-71a5-4011-9441-d0b5e4122711')
  })

  it('sets contentType to music and kind to artist', () => {
    const req = artistImportRequest(base)
    expect(req.contentType).toBe('music')
    expect(req.kind).toBe('artist')
  })

  it('defaults overview to empty string when not provided', () => {
    const req = artistImportRequest(base)
    expect(req.overview).toBe('')
  })

  it('preserves overview when provided', () => {
    const req = artistImportRequest({ ...base, overview: 'An English rock band.' })
    expect(req.overview).toBe('An English rock band.')
  })

  it('defaults to monitored with all monitor mode', () => {
    const req = artistImportRequest(base)
    expect(req.monitored).toBe(true)
    expect(req.monitorMode).toBe('all')
  })

  it('carries through imageUrl when present', () => {
    const req = artistImportRequest({ ...base, imageUrl: 'https://example.com/rh.jpg' })
    expect(req.imageUrl).toBe('https://example.com/rh.jpg')
  })

  it('leaves imageUrl undefined when not on candidate', () => {
    const req = artistImportRequest(base)
    expect(req.imageUrl).toBeUndefined()
  })
})
