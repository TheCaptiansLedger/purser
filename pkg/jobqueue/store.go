package jobqueue

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Store/Engine methods that can't find the
// requested Job.
var ErrNotFound = errors.New("jobqueue: not found")

// ErrUnknownKind is returned by Engine.Trigger when kind has no registered
// Executor.
var ErrUnknownKind = errors.New("jobqueue: unknown kind")

// Store is the persistence port pkg/jobqueue depends on internally —
// deliberately its own small interface rather than a use of
// internal/adapters/datastore's generic Datastore, since jobs are
// ephemeral and high-write-volume, the opposite profile Datastore is built
// for. See docs/adr/0023-job-queue.md. The in-memory implementation in
// pkg/jobqueue/memory is the default; a future pkg/jobqueue/redis package
// can implement this same interface without changing Engine or any caller.
//
// Every method operates on the whole Job aggregate (a Job and all its
// Tasks/Steps) — there is no separate Task/Step-level persistence, since a
// Job's Tasks/Steps are never meaningfully read or written independently
// of their parent Job.
type Store interface {
	// CreateJob persists a new Job. j.ID must not already exist.
	CreateJob(ctx context.Context, j *Job) error

	// UpdateJob persists j in place of the existing record with the same
	// ID, or returns ErrNotFound.
	UpdateJob(ctx context.Context, j *Job) error

	// GetJob returns the Job with the given ID, or ErrNotFound.
	GetJob(ctx context.Context, id string) (*Job, error)

	// ListJobs returns a page of Jobs ordered by creation time (oldest
	// first — UUIDv7 IDs sort chronologically), using opaque cursor
	// pagination: pageToken is the previous call's nextPageToken, empty
	// for the first page. pageSize <= 0 means the Store's own default.
	// kind/status are optional filters — empty kind or empty status means
	// no filter on that dimension. Filtering happens before pagination is
	// applied, so page boundaries never skip or repeat a matching Job.
	ListJobs(ctx context.Context, kind string, status Status, pageSize int, pageToken string) (jobs []*Job, nextPageToken string, err error)
}
