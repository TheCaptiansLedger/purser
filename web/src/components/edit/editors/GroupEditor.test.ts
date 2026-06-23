import { describe, expect, it } from 'vitest'
import { initialFormValues } from './GroupEditor'
import type { Group } from '../../../types'

const baseGroup: Group = {
  id: '1',
  libraryEntryId: 'entry-1',
  title: 'Test Album',
  sortName: 'Test Album',
  number: 1,
  year: 2023,
  overview: 'An overview',
  monitored: true,
  monitorMode: 'all',
  tags: [],
  lockedFields: [],
}

describe('initialFormValues', () => {
  it('maps group fields to form values', () => {
    const v = initialFormValues(baseGroup)
    expect(v.title).toBe('Test Album')
    expect(v.year).toBe('2023')
    expect(v.overview).toBe('An overview')
  })

  it('serialises year as empty string when year is 0', () => {
    expect(initialFormValues({ ...baseGroup, year: 0 }).year).toBe('')
  })

  it('falls back to empty string when overview is absent', () => {
    expect(initialFormValues({ ...baseGroup, overview: undefined as unknown as string }).overview).toBe('')
  })

  it('preserves title exactly', () => {
    expect(initialFormValues({ ...baseGroup, title: 'Abbey Road' }).title).toBe('Abbey Road')
  })
})
