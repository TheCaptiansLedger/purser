package service

import (
	"context"
	"purser/internal/ports"
	"purser/pkg/jobqueue"
)

// JobService orchestrates ports.JobPublisher and ports.JobReader — Job's
// service depends on both halves of the same capability (ISP-split by
// direction, per docs/adr/0023-job-queue.md), not a "God service" spanning
// entities.
type JobService struct {
	pub    ports.JobPublisher
	reader ports.JobReader
}

// NewJobService constructs a JobService backed by pub and reader.
func NewJobService(pub ports.JobPublisher, reader ports.JobReader) *JobService {
	return &JobService{pub: pub, reader: reader}
}

// Trigger starts a new Job of the given kind with one Task per label,
// running asynchronously, and returns its ID immediately.
func (s *JobService) Trigger(ctx context.Context, kind string, taskLabels []string) (string, error) {
	return s.pub.Trigger(ctx, kind, taskLabels)
}

// Get returns the full current state of the Job with the given id, or
// ports.ErrNotFound.
func (s *JobService) Get(ctx context.Context, id string) (*jobqueue.Job, error) {
	return s.reader.Get(ctx, id)
}
