import { describe, expect, it } from 'vitest'
import { albumMonitorLabel } from './AlbumCard'

describe('albumMonitorLabel', () => {
  it('returns the monitored tooltip when monitored', () => {
    expect(albumMonitorLabel(true)).toBe('Monitored — click to unmonitor')
  })

  it('returns the unmonitored tooltip when not monitored', () => {
    expect(albumMonitorLabel(false)).toBe('Unmonitored — click to monitor')
  })
})
