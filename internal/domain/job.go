package domain

import "time"

// JobStatus tracks the lifecycle state of an async background job.
type JobStatus string

// Job status constants covering the full async job lifecycle.
const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a single unit of async background work.
type Job struct {
	ID          string
	Name        string
	Payload     map[string]any
	Status      JobStatus
	Current     int
	Total       int
	Message     string
	Error       string
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// IsTerminal reports whether the job has reached a final state and will not
// transition further.
func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusFailed ||
		j.Status == JobStatusCancelled
}

// IsPending reports whether the job is still active (queued or running).
func (j *Job) IsPending() bool {
	return j.Status == JobStatusQueued || j.Status == JobStatusRunning
}
