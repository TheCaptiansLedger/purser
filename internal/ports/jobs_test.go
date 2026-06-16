package ports_test

import (
	"context"
	"errors"

	"purser/internal/domain"
	"purser/internal/ports"
)

// stubQueue verifies that a hand-rolled implementation satisfies JobQueue.
type stubQueue struct{}

func (s *stubQueue) Submit(_ context.Context, name string, _ map[string]any, _ ports.JobFunc) (*domain.Job, error) {
	return &domain.Job{Name: name, Status: domain.JobStatusQueued}, nil
}

func (s *stubQueue) Get(_ context.Context, id string) (*domain.Job, error) {
	if id == "" {
		return nil, errors.New("id required")
	}
	return &domain.Job{ID: id, Status: domain.JobStatusCompleted}, nil
}

func (s *stubQueue) List(_ context.Context) ([]*domain.Job, error) {
	return nil, nil
}

func (s *stubQueue) Cancel(_ context.Context, _ string) error {
	return nil
}

// stubReporter verifies that a hand-rolled implementation satisfies ProgressReporter.
type stubReporter struct {
	current, total int
	message        string
}

func (r *stubReporter) Report(current, total int, message string) {
	r.current = current
	r.total = total
	r.message = message
}

// Compile-time interface satisfaction checks.
var _ ports.JobQueue = (*stubQueue)(nil)
var _ ports.ProgressReporter = (*stubReporter)(nil)
