import { describe, expect, it } from 'vitest'
import { studioImportRequest } from './StudiosPage'
import type { ExternalStudio } from '../../types'

const base: ExternalStudio = {
  source: 'stashdb',
  externalId: 'e0f6e2d7-9a1c-4b8a-b9e4-3c2d1e0f5a6b',
  name: 'Evil Angel',
}

describe('studioImportRequest', () => {
  it('maps source and externalId from candidate', () => {
    const req = studioImportRequest(base)
    expect(req.source).toBe('stashdb')
    expect(req.externalId).toBe('e0f6e2d7-9a1c-4b8a-b9e4-3c2d1e0f5a6b')
  })

  it('sets contentType to adult and kind to studio', () => {
    const req = studioImportRequest(base)
    expect(req.contentType).toBe('adult')
    expect(req.kind).toBe('studio')
  })

  it('defaults overview to empty string when not provided', () => {
    const req = studioImportRequest(base)
    expect(req.overview).toBe('')
  })

  it('preserves overview when provided', () => {
    const req = studioImportRequest({ ...base, overview: 'An American adult studio.' })
    expect(req.overview).toBe('An American adult studio.')
  })

  it('defaults to monitored with latest monitor mode', () => {
    const req = studioImportRequest(base)
    expect(req.monitored).toBe(true)
    expect(req.monitorMode).toBe('latest')
  })

  it('carries through imageUrl when present', () => {
    const req = studioImportRequest({ ...base, imageUrl: 'https://example.com/ea.jpg' })
    expect(req.imageUrl).toBe('https://example.com/ea.jpg')
  })

  it('leaves imageUrl undefined when not on candidate', () => {
    const req = studioImportRequest(base)
    expect(req.imageUrl).toBeUndefined()
  })

  it('carries through parent fields when present', () => {
    const req = studioImportRequest({
      ...base,
      parentExternalId: 'parent-id',
      parentName: 'Parent Network',
    })
    expect(req.parentExternalId).toBe('parent-id')
    expect(req.parentName).toBe('Parent Network')
  })
})
