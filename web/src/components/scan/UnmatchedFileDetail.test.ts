import { describe, expect, it } from 'vitest'
import { candidateActionState } from './UnmatchedFileDetail'
import type { MatchCandidate } from '../../types'

function makeCandidate(overrides: Partial<MatchCandidate> = {}): MatchCandidate {
  return { item_id: '', confidence: 0.5, source: 'test', source_label: 'Test', ...overrides }
}

describe('candidateActionState', () => {
  it('canAccept when candidate has item_id', () => {
    expect(candidateActionState(makeCandidate({ item_id: 'item-1' })))
      .toEqual({ canAccept: true, canCreate: false })
  })

  it('canCreate when no item_id but has external parent', () => {
    const c = makeCandidate({
      external: {
        source: 'musicbrainz',
        external_id: 'rec-1',
        title: 'Track',
        content_type: 'music',
        parent: { source: 'musicbrainz', external_id: 'e-1', name: 'Artist' },
      },
    })
    expect(candidateActionState(c)).toEqual({ canAccept: false, canCreate: true })
  })

  it('neither action when no item_id and no parent', () => {
    const c = makeCandidate({
      external: { source: 'musicbrainz', external_id: 'rec-1', title: 'Track', content_type: 'music' },
    })
    expect(candidateActionState(c)).toEqual({ canAccept: false, canCreate: false })
  })

  it('neither action for undefined candidate', () => {
    expect(candidateActionState(undefined)).toEqual({ canAccept: false, canCreate: false })
  })
})
