import type { ReleaseStatus } from '../types'

export interface MusicReleaseStatusBadgeProps {
  status: ReleaseStatus
}

// LABEL_AND_COLOR_BY_STATUS maps a Music Release's status to its display
// label and the status/semantic color token bucket it belongs to, per
// docs/design/style-guide.md's "Entity → bucket mapping" table's
// `MusicRelease` row — before use, extend that table rather than
// hardcoding a switch over a fixed set (that doc's own warning). 'stub'
// maps to "Pending / not started" (known but nothing on disk yet);
// 'partial' maps to "Warning / partial / paused" — the same bucket a
// partial Job uses.
const LABEL_AND_COLOR_BY_STATUS: Record<ReleaseStatus, { label: string; className: string }> = {
  stub: { label: 'Stub', className: 'text-status-pending' },
  partial: { label: 'Partial', className: 'text-status-warning' },
  imported: { label: 'Imported', className: 'text-status-success' },
}

// MusicReleaseStatusBadge renders a Music Release's status as a short
// colored label — the same one-badge-per-entity pattern as
// JobStatusBadge/ItemStatusBadge, colocated alongside them rather than
// folded into a generic cross-entity StatusBadge.
export function MusicReleaseStatusBadge({ status }: MusicReleaseStatusBadgeProps) {
  const { label, className } = LABEL_AND_COLOR_BY_STATUS[status]
  return <span className={`inline-flex items-center text-label font-medium ${className}`}>{label}</span>
}
