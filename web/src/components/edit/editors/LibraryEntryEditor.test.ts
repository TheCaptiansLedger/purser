import { describe, expect, it } from 'vitest'
import { initialFormValues, STATUS_OPTIONS, MONITOR_MODE_OPTIONS } from './LibraryEntryEditor'
import type { LibraryEntry } from '../../../types'

const baseEntry: LibraryEntry = {
  id: '1',
  contentType: 'adult',
  kind: 'studio',
  name: 'Test Studio',
  sortName: 'Test Studio',
  overview: 'An overview',
  monitored: true,
  monitorMode: 'all',
  status: 'active',
  path: '/media/studios/test',
  externalIds: [{ source: 'stashdb', value: 'abc123' }],
  tags: [],
  people: [],
  lockedFields: [],
  addedAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-01T00:00:00Z',
}

describe('initialFormValues', () => {
  it('maps entry fields to form values', () => {
    const v = initialFormValues(baseEntry)
    expect(v.name).toBe('Test Studio')
    expect(v.sortName).toBe('Test Studio')
    expect(v.overview).toBe('An overview')
    expect(v.status).toBe('active')
    expect(v.monitorMode).toBe('all')
    expect(v.path).toBe('/media/studios/test')
    expect(v.externalIds).toEqual([{ source: 'stashdb', value: 'abc123' }])
  })

  it('serialises monitored as a string', () => {
    expect(initialFormValues({ ...baseEntry, monitored: true }).monitored).toBe('true')
    expect(initialFormValues({ ...baseEntry, monitored: false }).monitored).toBe('false')
  })

  it('falls back to empty string when path is absent', () => {
    expect(initialFormValues({ ...baseEntry, path: undefined }).path).toBe('')
  })

  it('defaults status to active when missing at runtime', () => {
    const v = initialFormValues({ ...baseEntry, status: undefined as unknown as 'active' })
    expect(v.status).toBe('active')
  })

  it('defaults monitorMode to all when missing at runtime', () => {
    const v = initialFormValues({ ...baseEntry, monitorMode: undefined as unknown as 'all' })
    expect(v.monitorMode).toBe('all')
  })

  it('falls back to empty array when externalIds is absent', () => {
    const v = initialFormValues({ ...baseEntry, externalIds: undefined as unknown as [] })
    expect(v.externalIds).toEqual([])
  })
})

describe('STATUS_OPTIONS', () => {
  it('covers all valid entry status values', () => {
    const values = STATUS_OPTIONS.map(o => o.value)
    expect(values).toContain('active')
    expect(values).toContain('continuing')
    expect(values).toContain('ended')
  })

  it('every option has a label', () => {
    for (const opt of STATUS_OPTIONS) {
      expect(opt.label.length).toBeGreaterThan(0)
    }
  })
})

describe('MONITOR_MODE_OPTIONS', () => {
  it('covers all valid monitor mode values', () => {
    const values = MONITOR_MODE_OPTIONS.map(o => o.value)
    expect(values).toContain('all')
    expect(values).toContain('future')
    expect(values).toContain('latest')
    expect(values).toContain('none')
  })

  it('every option has a label', () => {
    for (const opt of MONITOR_MODE_OPTIONS) {
      expect(opt.label.length).toBeGreaterThan(0)
    }
  })
})
