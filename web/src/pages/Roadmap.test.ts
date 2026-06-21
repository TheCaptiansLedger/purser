import { describe, expect, it } from 'vitest'
import { hasStatus, getQuarterKey, getIssueArea, parseIssueRefs, GHIssue } from './Roadmap'

// Helper mock issue creator
const createMockIssue = (labels: { name: string }[]): GHIssue => ({
  id: 1,
  number: 101,
  title: 'Mock Issue',
  html_url: 'https://github.com/mock',
  labels: labels.map((l, idx) => ({ id: idx, name: l.name, color: 'ffffff' })),
  user: { login: 'mockuser', avatar_url: 'https://avatar' },
  comments: 0,
  updated_at: '2026-06-17T00:00:00Z',
  closed_at: null,
  state: 'open'
})

describe('Roadmap Helper: hasStatus', () => {
  it('identifies status labels correctly', () => {
    const issue = createMockIssue([{ name: 'status: in-progress' }, { name: 'type: bug' }])
    expect(hasStatus(issue, 'in-progress')).toBe(true)
    expect(hasStatus(issue, 'ready')).toBe(false)
    expect(hasStatus(issue, 'blocked')).toBe(false)
  })

  it('returns false if no labels match', () => {
    const issue = createMockIssue([{ name: 'type: bug' }])
    expect(hasStatus(issue, 'ready')).toBe(false)
  })
})

describe('Roadmap Helper: getQuarterKey', () => {
  it('resolves correct quarter keys', () => {
    expect(getQuarterKey('2026-01-15T00:00:00Z')).toBe('2026-Q1')
    expect(getQuarterKey('2026-06-17T08:00:00Z')).toBe('2026-Q2')
    expect(getQuarterKey('2026-07-01T00:00:00Z')).toBe('2026-Q3')
    expect(getQuarterKey('2026-12-31T23:59:59Z')).toBe('2026-Q4')
  })
})

describe('Roadmap Helper: parseIssueRefs', () => {
  it('extracts issue numbers from markdown body', () => {
    expect(parseIssueRefs('Closes #42 and fixes #7')).toEqual([42, 7])
  })
  it('deduplicates repeated refs', () => {
    expect(parseIssueRefs('see #10, #10, #11')).toEqual([10, 11])
  })
  it('returns empty array for no refs', () => {
    expect(parseIssueRefs('No issue refs here')).toEqual([])
  })
  it('handles empty string', () => {
    expect(parseIssueRefs('')).toEqual([])
  })
})

describe('Roadmap Helper: getIssueArea', () => {
  it('extracts known area names correctly', () => {
    const issue = createMockIssue([{ name: 'area: ui' }])
    expect(getIssueArea(issue)).toBe('ui')

    const issueApi = createMockIssue([{ name: 'area: api' }])
    expect(getIssueArea(issueApi)).toBe('api')
  })

  it('defaults to other for unknown or missing areas', () => {
    const issueUnknown = createMockIssue([{ name: 'area: unknown-feature-area' }])
    expect(getIssueArea(issueUnknown)).toBe('other')

    const issueNone = createMockIssue([])
    expect(getIssueArea(issueNone)).toBe('other')
  })
})
