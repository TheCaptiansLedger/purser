import { describe, expect, it } from 'vitest'
import { getAvailableActions } from './ItemStatusPopover'

describe('getAvailableActions', () => {
  it('wanted: offers Skip only', () => {
    const actions = getAvailableActions('wanted')
    expect(actions.map(a => a.status)).toEqual(['skipped'])
  })

  it('skipped: offers Mark as Wanted only', () => {
    const actions = getAvailableActions('skipped')
    expect(actions.map(a => a.status)).toEqual(['wanted'])
  })

  it('missing: offers Mark as Wanted and Skip', () => {
    const statuses = getAvailableActions('missing').map(a => a.status)
    expect(statuses).toContain('wanted')
    expect(statuses).toContain('skipped')
    expect(statuses).toHaveLength(2)
  })

  it('imported: offers Mark as Wanted only (cannot be skipped)', () => {
    const actions = getAvailableActions('imported')
    expect(actions.map(a => a.status)).toEqual(['wanted'])
  })

  it('grabbed: no actions (pipeline-locked)', () => {
    expect(getAvailableActions('grabbed')).toHaveLength(0)
  })

  it('downloading: no actions (pipeline-locked)', () => {
    expect(getAvailableActions('downloading')).toHaveLength(0)
  })
})
