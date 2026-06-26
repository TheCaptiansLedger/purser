import { describe, expect, it } from 'vitest'
import { initialFormValues } from './PersonEditor'
import type { Person } from '../../../types'

const basePerson: Person = {
  id: 'p1',
  name: 'Jane Doe',
  sortName: 'Doe, Jane',
  overview: 'A performer.',
  monitored: true,
  monitorMode: 'all',
  aliases: ['JD'],
  externalIds: [{ source: 'stashdb', value: 'abc-123' }],
  metadata: { hair_color: 'brunette', height: 165 },
  lockedFields: [],
  addedAt: '2024-01-01T00:00:00Z',
}

describe('initialFormValues', () => {
  it('maps all person fields', () => {
    const v = initialFormValues(basePerson)
    expect(v.name).toBe('Jane Doe')
    expect(v.sortName).toBe('Doe, Jane')
    expect(v.overview).toBe('A performer.')
    expect(v.monitored).toBe(true)
    expect(v.monitorMode).toBe('all')
    expect(v.aliases).toEqual(['JD'])
    expect(v.externalIds).toEqual([{ source: 'stashdb', value: 'abc-123' }])
  })

  it('converts metadata to strings', () => {
    const v = initialFormValues(basePerson)
    expect(v.metadata).toEqual({ hair_color: 'brunette', height: '165' })
  })

  it('falls back to empty string for missing sortName', () => {
    const v = initialFormValues({ ...basePerson, sortName: undefined as unknown as string })
    expect(v.sortName).toBe('')
  })

  it('falls back to empty string for missing overview', () => {
    const v = initialFormValues({ ...basePerson, overview: undefined as unknown as string })
    expect(v.overview).toBe('')
  })

  it('falls back to empty arrays when person has none', () => {
    const v = initialFormValues({ ...basePerson, aliases: [], externalIds: [] })
    expect(v.aliases).toEqual([])
    expect(v.externalIds).toEqual([])
  })

  it('falls back to all monitor mode when missing', () => {
    const v = initialFormValues({ ...basePerson, monitorMode: undefined as unknown as 'all' })
    expect(v.monitorMode).toBe('all')
  })

  it('preserves monitored false', () => {
    expect(initialFormValues({ ...basePerson, monitored: false }).monitored).toBe(false)
  })

  it('produces empty metadata record when person has none', () => {
    const v = initialFormValues({ ...basePerson, metadata: undefined })
    expect(v.metadata).toEqual({})
  })
})
