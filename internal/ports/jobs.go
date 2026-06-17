package ports

import (
	"context"
	"purser/internal/domain"
)

// ProgressReporter is passed into a JobFunc so the running job can stream
// progress back to the queue without coupling to any concrete implementation.
type ProgressReporter interface {
	Report(current, total int, message string)
}

// JobFunc is the unit of work submitted to the queue.
// It receives a cancellable context and a ProgressReporter, and returns an
// error if the work fails. A nil error means the job completed successfully.
type JobFunc func(ctx context.Context, p ProgressReporter) error

// JobQueue manages the lifecycle of background jobs.
// The implementation is an in-memory worker pool; the interface is defined here
// so app services depend on the abstraction, not the goroutine machinery.
type JobQueue interface {
	// Submit enqueues a new job and returns it immediately with status=queued.
	// The job will be picked up by a free worker goroutine.
	Submit(ctx context.Context, name string, payload map[string]any, fn JobFunc) (*domain.Job, error)

	// Get returns the current state of a job by ID.
	Get(ctx context.Context, id string) (*domain.Job, error)

	// List returns all known jobs, most recent first.
	List(ctx context.Context) ([]*domain.Job, error)

	// Cancel requests cancellation of a queued or running job.
	// Has no effect if the job is already in a terminal state.
	Cancel(ctx context.Context, id string) error
}
