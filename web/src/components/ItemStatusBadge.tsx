import type { ItemStatus } from '../types'

export interface ItemStatusBadgeProps {
  status: ItemStatus
}

// LABEL_AND_COLOR_BY_STATUS maps an Item's status to its display label and
// the status/semantic color token bucket it belongs to, per
// docs/design/style-guide.md's "Entity → bucket mapping" table's `Item`
// row — before use, extend that table rather than hardcoding a switch over
// a fixed set (that doc's own warning).
const LABEL_AND_COLOR_BY_STATUS: Record<ItemStatus, { label: string; className: string }> = {
  wanted: { label: 'Wanted', className: 'text-status-pending' },
  grabbed: { label: 'Grabbed', className: 'text-status-queued' },
  downloading: { label: 'Downloading', className: 'text-status-active' },
  imported: { label: 'Imported', className: 'text-status-success' },
  missing: { label: 'Missing', className: 'text-status-failure' },
  skipped: { label: 'Skipped', className: 'text-status-neutral' },
}

// ItemStatusBadge renders an Item's status as a short colored label — the
// same one-badge-per-entity pattern as JobStatusBadge, colocated
// alongside it rather than folded into a generic cross-entity StatusBadge.
export function ItemStatusBadge({ status }: ItemStatusBadgeProps) {
  const { label, className } = LABEL_AND_COLOR_BY_STATUS[status]
  return <span className={`inline-flex items-center text-label font-medium ${className}`}>{label}</span>
}
