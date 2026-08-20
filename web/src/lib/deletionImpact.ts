import type { DeletionImpactRow } from '../gen/purser/domain/v1/common_pb'

// AggregatedImpactRow is the plain (non-wire) shape a bulk confirm dialog
// renders — same Kind/Label/Blocking/Count fields as DeletionImpactRow,
// but a plain object rather than a protobuf Message, since it's derived
// (summed across rows) rather than read directly off the wire.
export interface AggregatedImpactRow {
  kind: string
  label: string
  count: number
  blocking: boolean
}

// aggregateDeletionImpacts sums a bulk-select's per-row GetXDeletionImpact
// results into one list, grouped by Kind — the shape a bulk confirm dialog
// needs ("12 Groups total across the 3 artists you picked"), not a
// per-row breakdown. Kind/Label/Blocking are stable per Kind (every
// LibraryEntry's "group" row carries the same label/blocking-ness), so the
// first occurrence of a Kind wins for those, and only Count accumulates.
// Zero-count Kinds are dropped, same "don't show an empty row" precedent
// TrackDeleteDialog already set for the single-row case.
export function aggregateDeletionImpacts(perIdImpacts: DeletionImpactRow[][]): AggregatedImpactRow[] {
  const byKind = new Map<string, AggregatedImpactRow>()

  for (const rows of perIdImpacts) {
    for (const row of rows) {
      const existing = byKind.get(row.kind)
      if (existing) {
        existing.count += row.count
      } else {
        byKind.set(row.kind, { kind: row.kind, label: row.label, count: row.count, blocking: row.blocking })
      }
    }
  }

  return Array.from(byKind.values()).filter(row => row.count > 0)
}
