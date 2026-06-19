import { describe, expect, it } from 'vitest'
import type { EntryPerson, LibraryEntry, PersonRef } from './index'

describe('EntryPerson', () => {
  it('can be constructed with only required fields', () => {
    const ep: EntryPerson = { personId: 'person-1', role: 'member' }
    expect(ep.personId).toBe('person-1')
    expect(ep.role).toBe('member')
    expect(ep.startDate).toBeUndefined()
    expect(ep.endDate).toBeUndefined()
    expect(ep.person).toBeUndefined()
  })

  it('accepts an optional PersonRef stub', () => {
    const ref: PersonRef = { id: 'person-1', name: 'John Lennon', sortName: 'Lennon, John' }
    const ep: EntryPerson = { personId: ref.id, role: 'member', person: ref }
    expect(ep.person?.name).toBe('John Lennon')
    expect(ep.person?.sortName).toBe('Lennon, John')
  })

  it('accepts optional tenure dates', () => {
    const ep: EntryPerson = { personId: 'p-1', role: 'member', startDate: '1960-01-01', endDate: '1970-04-10' }
    expect(ep.startDate).toBe('1960-01-01')
    expect(ep.endDate).toBe('1970-04-10')
  })
})

describe('LibraryEntry.people', () => {
  it('is typed as an array of EntryPerson', () => {
    const people: LibraryEntry['people'] = [
      { personId: 'p-1', role: 'member' },
      { personId: 'p-2', role: 'vocalist' },
    ]
    expect(people).toHaveLength(2)
    expect(people[0].role).toBe('member')
  })

  it('accepts an empty array', () => {
    const people: LibraryEntry['people'] = []
    expect(people).toHaveLength(0)
  })
})
