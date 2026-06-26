import { describe, expect, it } from 'vitest'
import { initialFormValues } from './ItemEditor'
import type { Item } from '../../../types'

const baseItem: Item = {
  id: '1',
  contentType: 'adult',
  libraryEntryId: 'entry-1',
  title: 'Test Scene',
  overview: 'An overview',
  date: '2024-03-15',
  sequence: 'S01E02',
  runtimeSeconds: 3723,
  monitored: true,
  status: 'imported',
  people: [],
  tags: [],
  externalIds: [],
  lockedFields: [],
  addedAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-01T00:00:00Z',
}

describe('initialFormValues', () => {
  it('maps all item fields to form values', () => {
    const v = initialFormValues(baseItem)
    expect(v.title).toBe('Test Scene')
    expect(v.overview).toBe('An overview')
    expect(v.date).toBe('2024-03-15')
    expect(v.sequence).toBe('S01E02')
    expect(v.runtimeSeconds).toBe(3723)
    expect(v.monitored).toBe(true)
  })

  it('falls back to empty string when overview is absent', () => {
    expect(initialFormValues({ ...baseItem, overview: undefined as unknown as string }).overview).toBe('')
  })

  it('falls back to empty string when date is absent', () => {
    expect(initialFormValues({ ...baseItem, date: undefined }).date).toBe('')
  })

  it('falls back to empty string when sequence is absent', () => {
    expect(initialFormValues({ ...baseItem, sequence: undefined }).sequence).toBe('')
  })

  it('preserves monitored false', () => {
    expect(initialFormValues({ ...baseItem, monitored: false }).monitored).toBe(false)
  })

  it('preserves title exactly', () => {
    expect(initialFormValues({ ...baseItem, title: 'Studio Scene 42' }).title).toBe('Studio Scene 42')
  })
})
