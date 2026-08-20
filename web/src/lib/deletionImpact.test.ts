import { describe, expect, it } from 'vitest'
import type { DeletionImpactRow } from '../gen/purser/domain/v1/common_pb'
import { aggregateDeletionImpacts } from './deletionImpact'

function row(kind: string, count: number, blocking = false, label = kind): DeletionImpactRow {
  return { $typeName: 'purser.domain.v1.DeletionImpactRow', kind, label, count, blocking }
}

describe('aggregateDeletionImpacts', () => {
  it('sums Count per Kind across rows, keeping the first Label/Blocking seen', () => {
    const result = aggregateDeletionImpacts([
      [row('group', 2, true, 'Groups')],
      [row('group', 3, true, 'Groups'), row('image', 1)],
    ])

    expect(result).toEqual([
      { kind: 'group', label: 'Groups', count: 5, blocking: true },
      { kind: 'image', label: 'image', count: 1, blocking: false },
    ])
  })

  it('drops Kinds that sum to zero', () => {
    const result = aggregateDeletionImpacts([[row('image', 0)], [row('image', 0)]])

    expect(result).toEqual([])
  })
})
