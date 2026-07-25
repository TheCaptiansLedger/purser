// Package jobqueue is a generic, ephemeral, in-process engine for tracking
// long-running, multi-step operations (a directory scan, a batch import) at
// three levels of granularity: Job (the overall operation), Task (one unit
// of work within it, e.g. one file), and Step (one discrete action taken on
// a Task). It has zero imports of internal/domain or any other
// Purser-specific package — job Kinds and what a Task/Step represents are
// entirely caller-defined. See docs/adr/0023-job-queue.md.
package jobqueue

import (
	"time"

	"github.com/google/uuid"
)

// newID returns a new UUIDv7 identifier. This intentionally calls
// github.com/google/uuid directly rather than internal/domain.NewID() — the
// same underlying algorithm, but without importing anything Purser-specific,
// per docs/adr/0023-job-queue.md's ID-generation clarification.
func newID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Only fails if the system's entropy source is unusable — not a
		// condition any caller could meaningfully recover from.
		panic("jobqueue: generating UUIDv7: " + err.Error())
	}
	return id.String()
}

// Status is the lifecycle state shared by Job, Task, and Step.
type Status string

// The lifecycle states a Job/Task/Step moves through.
const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	// StatusPartial applies only to a Job (some Tasks succeeded, some
	// failed) — Task and Step never take this value themselves.
	StatusPartial Status = "partial"
)

// terminal reports whether s is a state a Job/Task/Step doesn't leave on
// its own.
func (s Status) terminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusPartial:
		return true
	default:
		return false
	}
}

// Step is one discrete action taken on a Task (e.g. "compute hashes",
// "query AcoustID"). Detail carries structured, step-specific results (a
// matched ID, a computed score) — the piece a log line alone can't give a
// caller polling the API.
type Step struct {
	ID         string
	Name       string
	Status     Status
	StartedAt  time.Time
	FinishedAt time.Time
	Message    string
	Detail     map[string]string
}

// Task is one unit of work within a Job (one file, one track). Label is
// what a caller displays (e.g. a filename).
type Task struct {
	ID         string
	Label      string
	Status     Status
	StartedAt  time.Time
	FinishedAt time.Time
	Steps      []*Step
}

// Progress returns the fraction (0..1) of t's Steps that have reached a
// terminal status. A Task with no Steps yet reports 0.
func (t *Task) Progress() float64 {
	if t == nil || len(t.Steps) == 0 {
		return 0
	}
	done := 0
	for _, s := range t.Steps {
		if s.Status.terminal() {
			done++
		}
	}
	return float64(done) / float64(len(t.Steps))
}

// Job is one overall operation (e.g. "scan /music/new-arrivals"). Kind is
// an open string the engine's executor registry dispatches on — this
// package has no built-in notion of what a "scan" is. Params is opaque,
// kind-specific trigger-time configuration: the engine passes it through
// unexamined, and only the registered Executor for Kind interprets its
// keys — the same "open, caller-defined" treatment Kind itself gets.
type Job struct {
	ID         string
	Kind       string
	Status     Status
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	Tasks      []*Task
	Params     map[string]string
}

// Progress returns the fraction (0..1) of j's Tasks that have reached a
// terminal status. A Job with no Tasks reports 0.
func (j *Job) Progress() float64 {
	if j == nil || len(j.Tasks) == 0 {
		return 0
	}
	done := 0
	for _, t := range j.Tasks {
		if t.Status.terminal() {
			done++
		}
	}
	return float64(done) / float64(len(j.Tasks))
}

// Clone returns a deep copy of j, so a caller mutating the result can never
// reach engine/store-owned state.
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}
	cp := *j
	if j.Params != nil {
		cp.Params = make(map[string]string, len(j.Params))
		for k, v := range j.Params {
			cp.Params[k] = v
		}
	}
	cp.Tasks = make([]*Task, len(j.Tasks))
	for i, t := range j.Tasks {
		tcp := *t
		tcp.Steps = make([]*Step, len(t.Steps))
		for k, s := range t.Steps {
			scp := *s
			if s.Detail != nil {
				scp.Detail = make(map[string]string, len(s.Detail))
				for dk, dv := range s.Detail {
					scp.Detail[dk] = dv
				}
			}
			tcp.Steps[k] = &scp
		}
		cp.Tasks[i] = &tcp
	}
	return &cp
}

// findTask returns the Task in j with the given id, or nil.
func findTask(j *Job, taskID string) *Task {
	for _, t := range j.Tasks {
		if t.ID == taskID {
			return t
		}
	}
	return nil
}

// findStep returns the Step in t with the given id, or nil.
func findStep(t *Task, stepID string) *Step {
	for _, s := range t.Steps {
		if s.ID == stepID {
			return s
		}
	}
	return nil
}
