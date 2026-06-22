import { describe, expect, it } from 'vitest'
import { lockButtonClass } from './FormField'

describe('lockButtonClass', () => {
  it('returns amber class when locked', () => {
    expect(lockButtonClass(true)).toContain('text-amber-400')
  })

  it('returns muted class when unlocked', () => {
    expect(lockButtonClass(false)).toContain('text-white/20')
  })

  it('locked and unlocked classes are different', () => {
    expect(lockButtonClass(true)).not.toBe(lockButtonClass(false))
  })
})
