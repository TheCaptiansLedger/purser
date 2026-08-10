// formatJobTimestamp renders a Job's created/started/finishedAt for the
// Jobs table (#606). '—' stands in for an unset timestamp (a pending Job
// has no startedAt; a running Job has no finishedAt) rather than leaving
// the cell blank — see docs/design/style-guide.md's Label-scale guidance
// for table cells.
export function formatJobTimestamp(date: Date | undefined): string {
  if (!date) return '—'
  return date.toLocaleString()
}

// formatJobProgress renders a Job's server-computed 0..1 progress fraction
// as a whole-number percentage, mirroring cmd/purser/jobs_format.go's
// formatProgress so the CLI and web table read consistently.
export function formatJobProgress(progress: number): string {
  return `${Math.round(progress * 100)}%`
}
