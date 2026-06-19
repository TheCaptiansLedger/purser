import { describe, expect, it } from 'vitest'
import type { LucideIcon } from 'lucide-react'
import { statusConfig } from './ItemStatusBadge'

describe('statusConfig', () => {
  it('returns a config for every known status', () => {
    const statuses = ['wanted', 'grabbed', 'downloading', 'imported', 'missing', 'skipped'] as const
    for (const s of statuses) {
      const cfg = statusConfig(s)
      expect(cfg.label).toBeTruthy()
      expect(cfg.color).toMatch(/^#[0-9a-f]{6}$/i)
      expect(cfg.icon).toBeDefined()
    }
  })

  it('icon is assignable to LucideIcon for every status', () => {
    // Compile-time guard: if the icon field type is ever narrowed away from
    // LucideIcon (e.g. a hand-rolled interface with size?: number), tsc will
    // fail here before the test even runs.
    const statuses = ['wanted', 'grabbed', 'downloading', 'imported', 'missing', 'skipped'] as const
    for (const s of statuses) {
      const icon: LucideIcon = statusConfig(s).icon
      expect(icon).toBeDefined()
    }
  })

  it('wanted has a blue color', () => {
    expect(statusConfig('wanted').color).toBe('#60a5fa')
  })

  it('imported has a green color', () => {
    expect(statusConfig('imported').color).toBe('#34d399')
  })

  it('missing has a red color', () => {
    expect(statusConfig('missing').color).toBe('#f87171')
  })

  it('skipped has a grey color', () => {
    expect(statusConfig('skipped').color).toBe('#9ca3af')
  })
})
