import { describe, expect, it } from 'vitest'
import {
  blankAlbumForm,
  releaseTypeFromExternal,
  releaseTypeToMetadata,
  toImportRequest,
  toCreateRequest,
} from './AddAlbumDialog'
import type { AlbumForm } from './AddAlbumDialog'

const baseForm: AlbumForm = {
  ...blankAlbumForm(),
  title: 'OK Computer',
  year: '1997',
}

const selected = { source: 'musicbrainz', externalId: 'e9a2e03f-9f25-4e81-863f-84eb00f56c07' }

// ── releaseTypeFromExternal ────────────────────────────────────────────────────

describe('releaseTypeFromExternal', () => {
  it('returns studio for a plain Album', () => {
    expect(releaseTypeFromExternal('Album', [])).toBe('studio')
  })

  it('returns live for Album + Live secondary', () => {
    expect(releaseTypeFromExternal('Album', ['Live'])).toBe('live')
  })

  it('returns compilation for Album + Compilation secondary', () => {
    expect(releaseTypeFromExternal('Album', ['Compilation'])).toBe('compilation')
  })

  it('returns ep_single for EP', () => {
    expect(releaseTypeFromExternal('EP', [])).toBe('ep_single')
  })

  it('returns ep_single for Single', () => {
    expect(releaseTypeFromExternal('Single', [])).toBe('ep_single')
  })

  it('returns other for unknown primary type', () => {
    expect(releaseTypeFromExternal('Broadcast', [])).toBe('other')
  })

  it('returns other when primaryType is absent', () => {
    expect(releaseTypeFromExternal(undefined, undefined)).toBe('other')
  })
})

// ── releaseTypeToMetadata ─────────────────────────────────────────────────────

describe('releaseTypeToMetadata', () => {
  it('maps studio to Album with empty secondary_types', () => {
    const m = releaseTypeToMetadata('studio')
    expect(m.primary_type).toBe('Album')
    expect(m.secondary_types).toEqual([])
  })

  it('maps live to Album + Live', () => {
    const m = releaseTypeToMetadata('live')
    expect(m.primary_type).toBe('Album')
    expect(m.secondary_types).toEqual(['Live'])
  })

  it('maps compilation to Album + Compilation', () => {
    const m = releaseTypeToMetadata('compilation')
    expect(m.secondary_types).toEqual(['Compilation'])
  })

  it('maps ep_single to EP', () => {
    const m = releaseTypeToMetadata('ep_single')
    expect(m.primary_type).toBe('EP')
  })

  it('returns empty object for other', () => {
    expect(releaseTypeToMetadata('other')).toEqual({})
  })
})

// ── toImportRequest ────────────────────────────────────────────────────────────

describe('toImportRequest', () => {
  it('maps source and externalId from selected album', () => {
    const req = toImportRequest(baseForm, 'entry-1', selected)
    expect(req.source).toBe('musicbrainz')
    expect(req.externalId).toBe('e9a2e03f-9f25-4e81-863f-84eb00f56c07')
  })

  it('maps title from form', () => {
    expect(toImportRequest(baseForm, 'entry-1', selected).title).toBe('OK Computer')
  })

  it('converts year string to number', () => {
    expect(toImportRequest(baseForm, 'entry-1', selected).year).toBe(1997)
  })

  it('omits year when blank', () => {
    expect(toImportRequest({ ...baseForm, year: '' }, 'entry-1', selected).year).toBeUndefined()
  })

  it('sets libraryEntryId', () => {
    expect(toImportRequest(baseForm, 'entry-123', selected).libraryEntryId).toBe('entry-123')
  })

  it('carries monitored and monitorMode', () => {
    const req = toImportRequest({ ...baseForm, monitored: false, monitorMode: 'future' }, 'entry-1', selected)
    expect(req.monitored).toBe(false)
    expect(req.monitorMode).toBe('future')
  })

  it('derives primaryType and secondaryTypes from releaseType', () => {
    const req = toImportRequest({ ...baseForm, releaseType: 'live' }, 'entry-1', selected)
    expect(req.primaryType).toBe('Album')
    expect(req.secondaryTypes).toEqual(['Live'])
  })

  it('sends no primaryType for other release type', () => {
    const req = toImportRequest({ ...baseForm, releaseType: 'other' }, 'entry-1', selected)
    expect(req.primaryType).toBeUndefined()
  })
})

// ── toCreateRequest ────────────────────────────────────────────────────────────

describe('toCreateRequest', () => {
  it('maps title and libraryEntryId', () => {
    const req = toCreateRequest(baseForm, 'entry-1')
    expect(req.title).toBe('OK Computer')
    expect(req.libraryEntryId).toBe('entry-1')
  })

  it('converts year string to number', () => {
    expect(toCreateRequest(baseForm, 'entry-1').year).toBe(1997)
  })

  it('omits year when blank', () => {
    expect(toCreateRequest({ ...baseForm, year: '' }, 'entry-1').year).toBeUndefined()
  })

  it('omits overview when blank', () => {
    expect(toCreateRequest(baseForm, 'entry-1').overview).toBeUndefined()
  })

  it('includes overview when provided', () => {
    expect(toCreateRequest({ ...baseForm, overview: 'A landmark album.' }, 'entry-1').overview).toBe('A landmark album.')
  })

  it('includes metadata derived from releaseType', () => {
    const req = toCreateRequest({ ...baseForm, releaseType: 'compilation' }, 'entry-1')
    expect(req.metadata?.primary_type).toBe('Album')
    expect(req.metadata?.secondary_types).toEqual(['Compilation'])
  })

  it('passes empty metadata object for other release type', () => {
    const req = toCreateRequest({ ...baseForm, releaseType: 'other' }, 'entry-1')
    expect(req.metadata).toEqual({})
  })
})
