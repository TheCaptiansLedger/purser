import { describe, expect, it } from 'vitest'
import { formatBytes } from './databaseFormat'

describe('formatBytes', () => {
  it('renders sub-KB counts in bytes', () => {
    expect(formatBytes(512)).toBe('512 B')
  })

  it('renders sub-MB counts in KB', () => {
    expect(formatBytes(2048)).toBe('2.0 KB')
  })

  it('renders sub-GB counts in MB', () => {
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB')
  })

  it('renders GB-scale counts in GB', () => {
    expect(formatBytes(3 * 1024 * 1024 * 1024)).toBe('3.0 GB')
  })

  it('renders zero as 0 B', () => {
    expect(formatBytes(0)).toBe('0 B')
  })
})
