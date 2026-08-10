import type { JobStatus } from '../types'

export interface JobStatusBadgeProps {
  status: JobStatus
}

// LABEL_AND_COLOR_BY_STATUS maps a Job's status to its display label and
// the status/semantic color token bucket it belongs to, per
// docs/design/style-guide.md's "Pending / not started" ... "Neutral /
// skipped / dismissed" table — before use, extend this table rather than
// hardcoding a switch over a fixed set (that doc's own warning).
// 'running' maps to "Active / in progress"; 'partial' maps to
// "Warning / partial / paused" — the same bucket cmd/purser/jobs_format.go
// colors partial jobs.
const LABEL_AND_COLOR_BY_STATUS: Record<JobStatus, { label: string; className: string }> = {
  unspecified: { label: 'Unknown', className: 'text-status-neutral' },
  pending: { label: 'Pending', className: 'text-status-pending' },
  running: { label: 'Running', className: 'text-status-active' },
  succeeded: { label: 'Succeeded', className: 'text-status-success' },
  failed: { label: 'Failed', className: 'text-status-failure' },
  partial: { label: 'Partial', className: 'text-status-warning' },
}

// JobStatusBadge renders a Job's status as a short colored label — the
// Jobs table's (#606) Status column. Colocated in components/ alongside
// LockBadge as a small, reusable status atom rather than folded into
// JobsTab itself.
export function JobStatusBadge({ status }: JobStatusBadgeProps) {
  const { label, className } = LABEL_AND_COLOR_BY_STATUS[status]
  return <span className={`inline-flex items-center text-label font-medium ${className}`}>{label}</span>
}
