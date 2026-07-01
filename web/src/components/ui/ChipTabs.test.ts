import { describe, expect, it } from 'vitest'
import { chipTabClassName, chipTabStyle } from './ChipTabs'

describe('chipTabClassName', () => {
  it('includes base layout classes regardless of active state', () => {
    expect(chipTabClassName(true)).toContain('flex items-center gap-1.5')
    expect(chipTabClassName(false)).toContain('flex items-center gap-1.5')
  })

  it('active tab gets text-white', () => {
    expect(chipTabClassName(true)).toContain('text-white')
  })

  it('inactive tab gets muted text and hover classes', () => {
    const cls = chipTabClassName(false)
    expect(cls).toContain('text-white/40')
    expect(cls).toContain('hover:text-white/65')
    expect(cls).toContain('hover:bg-white/5')
  })

  it('active tab does not get inactive hover classes', () => {
    expect(chipTabClassName(true)).not.toContain('hover:bg-white/5')
  })
})

describe('ChipTab icon field', () => {
  it('icon is optional — tab without icon is a valid ChipTab', () => {
    const tab = { id: 'all', label: 'All' }
    expect(tab.id).toBe('all')
    expect(tab.label).toBe('All')
    expect('icon' in tab).toBe(false)
  })
})

describe('chipTabStyle', () => {
  it('active tab gets accent background and color', () => {
    const style = chipTabStyle(true, '#f43f5e')
    expect(style.background).toBe('#f43f5e28')
    expect(style.color).toBe('#f43f5e')
  })

  it('inactive tab returns empty style object', () => {
    expect(chipTabStyle(false, '#f43f5e')).toEqual({})
  })

  it('accent color is used verbatim', () => {
    const style = chipTabStyle(true, '#10b981')
    expect(style.color).toBe('#10b981')
    expect(style.background).toBe('#10b98128')
  })
})
