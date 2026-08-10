import { describe, expect, it } from 'vitest'
import { encodeSettingValue, parseSettingValue } from './settingValue'

describe('parseSettingValue', () => {
  it('JSON-parses a non-secret value', () => {
    expect(parseSettingValue({ value: '"/data/media"', secret: false })).toBe('/data/media')
    expect(parseSettingValue({ value: 'true', secret: false })).toBe(true)
    expect(parseSettingValue({ value: '["/roots/a","/roots/b"]', secret: false })).toEqual(['/roots/a', '/roots/b'])
  })

  it('treats an unset secret as an empty string, never JSON.parse-ing it', () => {
    expect(parseSettingValue({ value: '', secret: true })).toBe('')
  })

  it('treats a set secret as the literal masked placeholder, never JSON.parse-ing it', () => {
    expect(parseSettingValue({ value: '********', secret: true })).toBe('********')
  })
})

describe('encodeSettingValue', () => {
  it('JSON-encodes a plain string the same for secret and non-secret fields', () => {
    expect(encodeSettingValue('new-api-key')).toBe('"new-api-key"')
  })

  it('JSON-encodes a string array', () => {
    expect(encodeSettingValue(['/roots/a'])).toBe('["/roots/a"]')
  })
})
