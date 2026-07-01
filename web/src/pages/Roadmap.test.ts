import { describe, expect, it } from 'vitest'
import { hasStatus, getQuarterKey, getIssueArea, parseIssueRefs, sortedQuarterKeys, uniqueContributors, issuesInWindow, GHIssue, GHRelease } from './Roadmap'

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

describe('uniqueContributors', () => {
  const user = (login: string) => ({ login, avatar_url: `https://avatar/${login}` })

  it('returns unique contributors in first-seen order', () => {
    const commits = [
      { author: user('alice') },
      { author: user('bob') },
      { author: user('alice') },
    ]
    const result = uniqueContributors(commits)
    expect(result.map(u => u.login)).toEqual(['alice', 'bob'])
  })

  it('skips commits with no linked GitHub account', () => {
    const commits = [
      { author: null },
      { author: user('carol') },
      { author: null },
    ]
    const result = uniqueContributors(commits)
    expect(result.map(u => u.login)).toEqual(['carol'])
  })

  it('returns empty array for empty input', () => {
    expect(uniqueContributors([])).toEqual([])
  })
})

describe('sortedQuarterKeys', () => {
  const rel = (published_at: string): GHRelease => ({
    id: 1, tag_name: 'v0.1', name: 'v0.1', html_url: '', published_at, body: '',
  })

  it('returns quarter keys sorted latest first', () => {
    const releases = [
      rel('2026-01-15T00:00:00Z'),
      rel('2026-06-01T00:00:00Z'),
      rel('2025-09-01T00:00:00Z'),
    ]
    expect(sortedQuarterKeys(releases)).toEqual(['2026-Q2', '2026-Q1', '2025-Q3'])
  })

  it('deduplicates releases in the same quarter', () => {
    const releases = [
      rel('2026-01-01T00:00:00Z'),
      rel('2026-02-15T00:00:00Z'),
    ]
    expect(sortedQuarterKeys(releases)).toEqual(['2026-Q1'])
  })

  it('returns empty array for no releases', () => {
    expect(sortedQuarterKeys([])).toEqual([])
  })
})

describe('issuesInWindow', () => {
  const closedIssue = (closed_at: string, labels: { name: string }[] = []): GHIssue => ({
    ...createMockIssue(labels),
    closed_at,
    state: 'closed',
  })

  it('returns issues closed within the window', () => {
    const candidates = [
      closedIssue('2026-03-15T00:00:00Z'),
      closedIssue('2026-04-01T00:00:00Z'),
    ]
    expect(issuesInWindow(candidates, '2026-03-01T00:00:00Z', '2026-04-01T00:00:00Z')).toHaveLength(2)
  })

  it('excludes issues closed before prevPublishedAt', () => {
    const candidates = [closedIssue('2026-02-28T00:00:00Z')]
    expect(issuesInWindow(candidates, '2026-03-01T00:00:00Z', '2026-04-01T00:00:00Z')).toHaveLength(0)
  })

  it('excludes issues closed after thisPublishedAt', () => {
    const candidates = [closedIssue('2026-05-01T00:00:00Z')]
    expect(issuesInWindow(candidates, '2026-03-01T00:00:00Z', '2026-04-01T00:00:00Z')).toHaveLength(0)
  })

  it('thisPublishedAt boundary is inclusive', () => {
    const candidates = [closedIssue('2026-04-01T00:00:00Z')]
    expect(issuesInWindow(candidates, '2026-03-01T00:00:00Z', '2026-04-01T00:00:00Z')).toHaveLength(1)
  })

  it('prevPublishedAt boundary is exclusive', () => {
    const candidates = [closedIssue('2026-03-01T00:00:00Z')]
    expect(issuesInWindow(candidates, '2026-03-01T00:00:00Z', '2026-04-01T00:00:00Z')).toHaveLength(0)
  })

  it('null prevPublishedAt matches all issues up to thisPublishedAt', () => {
    const candidates = [
      closedIssue('2020-01-01T00:00:00Z'),
      closedIssue('2026-03-15T00:00:00Z'),
    ]
    expect(issuesInWindow(candidates, null, '2026-04-01T00:00:00Z')).toHaveLength(2)
  })

  it('excludes issues with null closed_at', () => {
    const open: GHIssue = { ...createMockIssue([]), closed_at: null, state: 'open' }
    expect(issuesInWindow([open], null, '2026-04-01T00:00:00Z')).toHaveLength(0)
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
