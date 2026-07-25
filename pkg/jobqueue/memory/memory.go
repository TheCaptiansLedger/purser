// Package memory is the in-memory adapter for the jobqueue.Store port
// defined in pkg/jobqueue — the default backend, ephemeral by design (job
// history does not survive a restart). See docs/adr/0023-job-queue.md.
package memory

import (
	"context"
	"fmt"
	"purser/pkg/jobqueue"
	"sync"
)

// defaultPageSize is used when a caller passes pageSize <= 0.
const defaultPageSize = 50

// Store is the in-memory jobqueue.Store adapter.
type Store struct {
	mu    sync.RWMutex
	jobs  map[string]*jobqueue.Job
	order []string // insertion order, i.e. creation order
}

// New constructs an empty in-memory Store.
func New() *Store {
	return &Store{jobs: make(map[string]*jobqueue.Job)}
}

// CreateJob implements jobqueue.Store.
func (s *Store) CreateJob(_ context.Context, j *jobqueue.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[j.ID]; exists {
		return fmt.Errorf("jobqueue/memory: job %q already exists", j.ID)
	}
	s.jobs[j.ID] = j.Clone()
	s.order = append(s.order, j.ID)
	return nil
}

// UpdateJob implements jobqueue.Store.
func (s *Store) UpdateJob(_ context.Context, j *jobqueue.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[j.ID]; !exists {
		return jobqueue.ErrNotFound
	}
	s.jobs[j.ID] = j.Clone()
	return nil
}

// GetJob implements jobqueue.Store.
func (s *Store) GetJob(_ context.Context, id string) (*jobqueue.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	j, ok := s.jobs[id]
	if !ok {
		return nil, jobqueue.ErrNotFound
	}
	return j.Clone(), nil
}

// ListJobs implements jobqueue.Store, paginating over insertion order.
// pageToken is the ID of the last job returned by the previous page, or
// empty for the first page.
func (s *Store) ListJobs(_ context.Context, pageSize int, pageToken string) ([]*jobqueue.Job, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	start := 0
	if pageToken != "" {
		for i, id := range s.order {
			if id == pageToken {
				start = i + 1
				break
			}
		}
	}

	if start >= len(s.order) {
		return nil, "", nil
	}

	end := min(start+pageSize, len(s.order))
	page := make([]*jobqueue.Job, 0, end-start)
	for _, id := range s.order[start:end] {
		page = append(page, s.jobs[id].Clone())
	}

	nextPageToken := ""
	if end < len(s.order) {
		nextPageToken = s.order[end-1]
	}
	return page, nextPageToken, nil
}
