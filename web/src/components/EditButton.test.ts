import { describe, expect, it } from 'vitest'
import { editButtonClass } from './EditButton'

describe('editButtonClass', () => {
  it('returns base classes without extra', () => {
    const cls = editButtonClass()
    expect(cls).toContain('inline-flex')
    expect(cls).toContain('text-white/50')
    expect(cls).toContain('border-white/10')
  })

  it('appends extra class when provided', () => {
    expect(editButtonClass('shrink-0')).toContain('shrink-0')
  })

  it('base and extra are space-separated', () => {
    expect(editButtonClass('shrink-0')).toMatch(/transition-colors shrink-0$/)
  })

  it('does not add trailing space without extra', () => {
    expect(editButtonClass()).not.toMatch(/\s$/)
  })

  it('extra class does not appear without argument', () => {
    expect(editButtonClass()).not.toContain('shrink-0')
  })
})
