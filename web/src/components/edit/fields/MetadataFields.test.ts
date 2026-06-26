import { describe, expect, it } from 'vitest'
import { toMetadataStrings } from './MetadataFields'

describe('toMetadataStrings', () => {
  it('converts string values unchanged', () => {
    expect(toMetadataStrings({ hair_color: 'brown' })).toEqual({ hair_color: 'brown' })
  })

  it('converts numbers to strings', () => {
    expect(toMetadataStrings({ height: 165 })).toEqual({ height: '165' })
  })

  it('converts booleans to strings', () => {
    expect(toMetadataStrings({ active: true })).toEqual({ active: 'true' })
  })

  it('converts null to empty string', () => {
    expect(toMetadataStrings({ key: null })).toEqual({ key: '' })
  })

  it('converts undefined to empty string', () => {
    expect(toMetadataStrings({ key: undefined })).toEqual({ key: '' })
  })

  it('filters out array values', () => {
    expect(toMetadataStrings({ types: ['a', 'b'], name: 'x' })).toEqual({ name: 'x' })
  })

  it('filters out object values', () => {
    expect(toMetadataStrings({ nested: { a: 1 }, name: 'x' })).toEqual({ name: 'x' })
  })

  it('returns empty object for empty input', () => {
    expect(toMetadataStrings({})).toEqual({})
  })

  it('handles mixed scalar and non-scalar values', () => {
    expect(toMetadataStrings({
      hair_color: 'blonde',
      height: 170,
      tags: ['a', 'b'],
    })).toEqual({ hair_color: 'blonde', height: '170' })
  })
})
