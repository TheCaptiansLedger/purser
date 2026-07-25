package ports

import (
	"context"
	"purser/pkg/jobqueue"
)

// JobPublisher lets service/pipeline code create and drive a Job through
// pkg/jobqueue's engine, without importing pkg/jobqueue directly. This
// issue's slice only needs Trigger — a later sub-issue widens this
// interface to let pipeline code create a Job and push its own step
// updates directly, rather than only dispatching to a registered
// executor by Kind. See docs/adr/0023-job-queue.md.
type JobPublisher interface {
	// Trigger starts a new Job of the given kind with one Task per label,
	// running asynchronously, and returns its ID immediately. params is
	// opaque, kind-specific configuration passed through unexamined to the
	// registered Executor.
	Trigger(ctx context.Context, kind string, taskLabels []string, params map[string]string) (string, error)
}

// JobReader lets the API layer read Job state without importing
// pkg/jobqueue directly. Watch is added by a later sub-issue. See
// docs/adr/0023-job-queue.md.
type JobReader interface {
	// Get returns the full current state of the Job (all Tasks, all
	// Steps), or ErrNotFound.
	Get(ctx context.Context, id string) (*jobqueue.Job, error)

	// List returns a page of Jobs, optionally filtered by kind and/or
	// status (empty kind / empty status means no filter on that
	// dimension), using opaque cursor pagination per
	// docs/adr/0011-api-design.md.
	List(ctx context.Context, kind string, status jobqueue.Status, pageSize int, pageToken string) (jobs []*jobqueue.Job, nextPageToken string, err error)
}
