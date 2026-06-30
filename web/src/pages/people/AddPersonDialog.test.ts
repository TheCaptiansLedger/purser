import { describe, expect, it } from 'vitest'
import { blankPersonForm, toCreateRequest } from './AddPersonDialog'
import type { PersonForm } from './AddPersonDialog'

// ── toCreateRequest ────────────────────────────────────────────────────────────

describe('toCreateRequest', () => {
  it('maps name and monitoring fields', () => {
    const form: PersonForm = { ...blankPersonForm(), name: 'Jane Smith' }
    const req = toCreateRequest(form)
    expect(req.name).toBe('Jane Smith')
    expect(req.monitored).toBe(true)
    expect(req.monitorMode).toBe('all')
  })

  it('omits sortName when blank', () => {
    const req = toCreateRequest({ ...blankPersonForm(), name: 'Jane' })
    expect(req.sortName).toBeUndefined()
  })

  it('includes sortName when provided', () => {
    const req = toCreateRequest({ ...blankPersonForm(), name: 'Jane', sortName: 'Smith, Jane' })
    expect(req.sortName).toBe('Smith, Jane')
  })

  it('omits overview when blank', () => {
    const req = toCreateRequest({ ...blankPersonForm(), name: 'Jane' })
    expect(req.overview).toBeUndefined()
  })

  it('omits aliases when empty', () => {
    const req = toCreateRequest({ ...blankPersonForm(), name: 'Jane' })
    expect(req.aliases).toBeUndefined()
  })

  it('includes aliases when present', () => {
    const req = toCreateRequest({ ...blankPersonForm(), name: 'Jane', aliases: ['J. Smith'] })
    expect(req.aliases).toEqual(['J. Smith'])
  })

  it('omits roles when none selected', () => {
    const req = toCreateRequest({ ...blankPersonForm(), name: 'Jane' })
    expect(req.roles).toBeUndefined()
  })

  it('includes roles when selected', () => {
    const form: PersonForm = { ...blankPersonForm(), name: 'Jane', roles: ['performer', 'actress'] }
    const req = toCreateRequest(form)
    expect(req.roles).toEqual(['performer', 'actress'])
  })

  it('omits externalIds when empty', () => {
    const req = toCreateRequest({ ...blankPersonForm(), name: 'Jane' })
    expect(req.externalIds).toBeUndefined()
  })

  it('includes externalIds when present', () => {
    const form: PersonForm = {
      ...blankPersonForm(),
      name: 'Jane',
      externalIds: [{ source: 'stashdb', value: 'abc-123' }],
    }
    const req = toCreateRequest(form)
    expect(req.externalIds).toEqual([{ source: 'stashdb', value: 'abc-123' }])
  })
})
