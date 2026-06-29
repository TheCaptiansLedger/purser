import { describe, expect, it } from 'vitest'
import { expandableTextStyle } from './ExpandableText'

describe('expandableTextStyle', () => {
  it('returns undefined when expanded', () => {
    expect(expandableTextStyle(true, 4)).toBeUndefined()
  })

  it('returns line-clamp styles when collapsed', () => {
    const style = expandableTextStyle(false, 4)
    expect(style?.WebkitLineClamp).toBe(4)
    expect(style?.overflow).toBe('hidden')
    expect(style?.display).toBe('-webkit-box')
  })

  it('uses the provided maxLines value', () => {
    expect(expandableTextStyle(false, 6)?.WebkitLineClamp).toBe(6)
  })
})
