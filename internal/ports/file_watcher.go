package ports

import "purser/pkg/fswatch"

// FileWatcher lets service code consume live filesystem discovery events
// without depending on pkg/fswatch's concrete watcher-construction/source
// wiring — the same seam docs/adr/0023-job-queue.md established for
// pkg/jobqueue (JobPublisher/JobReader), applied here to the other half of
// docs/adr/0024-pipeline-core.md's "Discovery: both a watcher and an
// on-demand recursive scan, one code path." *fswatch.Watcher satisfies
// this directly — no adapter wrapper is needed, since pkg/fswatch is
// already generic, Purser-agnostic infrastructure.
type FileWatcher interface {
	// Events returns the channel of settled unit events. Closed once the
	// underlying watcher stops.
	Events() <-chan fswatch.Event

	// Errors returns the channel of asynchronous watch errors. Closed once
	// the underlying watcher stops.
	Errors() <-chan error
}
