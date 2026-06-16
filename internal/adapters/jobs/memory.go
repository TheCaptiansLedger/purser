package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
)

type jobEntry struct {
	job    *domain.Job
	cancel context.CancelFunc
}

type workItem struct {
	jobID string
	ctx   context.Context
	fn    ports.JobFunc
}

// Queue is an in-memory worker pool implementing ports.JobQueue.
type Queue struct {
	mu   sync.RWMutex
	jobs map[string]*jobEntry
	ch   chan workItem
	wg   sync.WaitGroup
}

// New returns a Queue backed by workerCount goroutines. Call Close when done.
func New(workerCount int) *Queue {
	q := &Queue{
		jobs: make(map[string]*jobEntry),
		ch:   make(chan workItem, workerCount*4),
	}
	q.wg.Add(workerCount)
	for range workerCount {
		go q.worker()
	}
	return q
}

// Close stops accepting new work and waits for all in-flight jobs to finish.
func (q *Queue) Close() {
	close(q.ch)
	q.wg.Wait()
}

func (q *Queue) Submit(ctx context.Context, name string, payload map[string]any, fn ports.JobFunc) (*domain.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	jobCtx, cancel := context.WithCancel(context.Background())

	job := &domain.Job{
		ID:        uuid.New().String(),
		Name:      name,
		Payload:   payload,
		Status:    domain.JobStatusQueued,
		CreatedAt: time.Now().UTC(),
	}

	q.mu.Lock()
	q.jobs[job.ID] = &jobEntry{job: job, cancel: cancel}
	snap := copyJob(job) // copy before the worker can mutate
	q.mu.Unlock()

	q.ch <- workItem{jobID: job.ID, ctx: jobCtx, fn: fn}

	return snap, nil
}

func (q *Queue) Get(_ context.Context, id string) (*domain.Job, error) {
	q.mu.RLock()
	entry, ok := q.jobs[id]
	var snap *domain.Job
	if ok {
		snap = copyJob(entry.job)
	}
	q.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("job %s: %w", id, errs.ErrNotFound)
	}
	return snap, nil
}

func (q *Queue) List(_ context.Context) ([]*domain.Job, error) {
	q.mu.RLock()
	out := make([]*domain.Job, 0, len(q.jobs))
	for _, entry := range q.jobs {
		out = append(out, copyJob(entry.job))
	}
	q.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (q *Queue) Cancel(_ context.Context, id string) error {
	q.mu.Lock()
	entry, ok := q.jobs[id]
	if !ok {
		q.mu.Unlock()
		return fmt.Errorf("job %s: %w", id, errs.ErrNotFound)
	}
	entry.cancel()
	// Flip queued jobs to cancelled immediately so the next poll reflects
	// reality before the worker drains the channel.
	if entry.job.Status == domain.JobStatusQueued {
		now := time.Now().UTC()
		entry.job.Status = domain.JobStatusCancelled
		entry.job.CompletedAt = &now
	}
	q.mu.Unlock()
	return nil
}

func (q *Queue) worker() {
	defer q.wg.Done()
	for item := range q.ch {
		// Skip jobs already terminal (e.g., cancelled while still queued).
		q.mu.RLock()
		entry := q.jobs[item.jobID]
		terminal := entry != nil && entry.job.IsTerminal()
		q.mu.RUnlock()
		if terminal {
			continue
		}
		q.setRunning(item.jobID)
		p := &progressReporter{queue: q, jobID: item.jobID}
		err := item.fn(item.ctx, p)
		q.setTerminal(item.jobID, err)
	}
}

func (q *Queue) setRunning(jobID string) {
	now := time.Now().UTC()
	q.mu.Lock()
	entry := q.jobs[jobID]
	entry.job.Status = domain.JobStatusRunning
	entry.job.StartedAt = &now
	name := entry.job.Name
	q.mu.Unlock()
	slog.Info("job.started", "job_id", jobID, "job_name", name)
}

func (q *Queue) setTerminal(jobID string, err error) {
	now := time.Now().UTC()
	q.mu.Lock()
	entry := q.jobs[jobID]
	entry.job.CompletedAt = &now
	var status domain.JobStatus
	var errMsg string
	if err == nil {
		status = domain.JobStatusCompleted
	} else if errors.Is(err, context.Canceled) {
		status = domain.JobStatusCancelled
	} else {
		status = domain.JobStatusFailed
		errMsg = err.Error()
	}
	entry.job.Status = status
	entry.job.Error = errMsg
	name := entry.job.Name
	q.mu.Unlock()

	switch status {
	case domain.JobStatusCompleted:
		slog.Info("job.completed", "job_id", jobID, "job_name", name)
	case domain.JobStatusCancelled:
		slog.Info("job.cancelled", "job_id", jobID, "job_name", name)
	default:
		slog.Error("job.failed", "job_id", jobID, "job_name", name, "error", errMsg)
	}
}

type progressReporter struct {
	queue *Queue
	jobID string
}

func (p *progressReporter) Report(current, total int, message string) {
	p.queue.mu.Lock()
	entry := p.queue.jobs[p.jobID]
	entry.job.Current = current
	entry.job.Total = total
	entry.job.Message = message
	p.queue.mu.Unlock()
}

func copyJob(j *domain.Job) *domain.Job {
	cp := *j
	if j.Payload != nil {
		cp.Payload = make(map[string]any, len(j.Payload))
		for k, v := range j.Payload {
			cp.Payload[k] = v
		}
	}
	if j.StartedAt != nil {
		t := *j.StartedAt
		cp.StartedAt = &t
	}
	if j.CompletedAt != nil {
		t := *j.CompletedAt
		cp.CompletedAt = &t
	}
	return &cp
}

// Compile-time interface check.
var _ ports.JobQueue = (*Queue)(nil)
