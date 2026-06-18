import { describe, expect, it } from 'vitest'
import { jobTimestamp } from './JobsPanel'
import type { Job } from '../../types'

const base: Job = {
  id: 'job-1',
  name: 'test.job',
  status: 'queued',
  current: 0,
  total: 0,
  createdAt: '2026-06-18T10:00:00Z',
}

describe('jobTimestamp', () => {
  it('returns queued timestamp for queued jobs', () => {
    expect(jobTimestamp({ ...base, status: 'queued' })).toEqual({
      label: 'queued',
      ts: '2026-06-18T10:00:00Z',
    })
  })

  it('returns started timestamp for running jobs with startedAt', () => {
    expect(jobTimestamp({ ...base, status: 'running', startedAt: '2026-06-18T10:01:00Z' })).toEqual({
      label: 'started',
      ts: '2026-06-18T10:01:00Z',
    })
  })

  it('returns null for running jobs without startedAt', () => {
    expect(jobTimestamp({ ...base, status: 'running' })).toBeNull()
  })

  it('returns finished timestamp for completed jobs', () => {
    expect(jobTimestamp({ ...base, status: 'completed', completedAt: '2026-06-18T10:05:00Z' })).toEqual({
      label: 'finished',
      ts: '2026-06-18T10:05:00Z',
    })
  })

  it('returns finished timestamp for failed jobs with completedAt', () => {
    expect(jobTimestamp({ ...base, status: 'failed', completedAt: '2026-06-18T10:02:00Z' })).toEqual({
      label: 'finished',
      ts: '2026-06-18T10:02:00Z',
    })
  })

  it('returns null for failed jobs without completedAt', () => {
    expect(jobTimestamp({ ...base, status: 'failed' })).toBeNull()
  })

  it('returns null for cancelled jobs without completedAt', () => {
    expect(jobTimestamp({ ...base, status: 'cancelled' })).toBeNull()
  })

  it('returns finished timestamp for cancelled jobs with completedAt', () => {
    expect(jobTimestamp({ ...base, status: 'cancelled', completedAt: '2026-06-18T10:03:00Z' })).toEqual({
      label: 'finished',
      ts: '2026-06-18T10:03:00Z',
    })
  })
})
