import { describe, expect, it } from 'vitest'
import { initialFormValues } from './ItemEditor'
import type { Item } from '../../../types'

const baseItem: Item = {
  id: '1',
  contentType: 'adult',
  libraryEntryId: 'entry-1',
  title: 'Test Scene',
  overview: 'An overview',
  runtimeSeconds: 3600,
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
  it('maps item fields to form values', () => {
    const v = initialFormValues(baseItem)
    expect(v.title).toBe('Test Scene')
    expect(v.overview).toBe('An overview')
  })

  it('falls back to empty string when overview is absent', () => {
    expect(initialFormValues({ ...baseItem, overview: undefined as unknown as string }).overview).toBe('')
  })

  it('preserves title exactly', () => {
    expect(initialFormValues({ ...baseItem, title: 'Studio Scene 42' }).title).toBe('Studio Scene 42')
  })
})
