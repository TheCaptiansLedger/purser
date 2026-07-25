// Package jobqueue is the thin adapter implementing ports.JobPublisher and
// ports.JobReader by wrapping a pkg/jobqueue.Engine — the hexagonal seam
// pkg/jobqueue is consumed through, per docs/adr/0023-job-queue.md and
// docs/adr/0001-hexagonal-architecture.md. One concrete type satisfies both
// narrow ports; ISP is about interface width, not concrete-type count.
package jobqueue

import (
	"context"
	"errors"
	"purser/internal/ports"

	pkgjobqueue "purser/pkg/jobqueue"
)

// Adapter implements ports.JobPublisher and ports.JobReader.
type Adapter struct {
	engine *pkgjobqueue.Engine
}

// New constructs an Adapter backed by engine.
func New(engine *pkgjobqueue.Engine) *Adapter {
	return &Adapter{engine: engine}
}

// Trigger implements ports.JobPublisher.
func (a *Adapter) Trigger(ctx context.Context, kind string, taskLabels []string) (string, error) {
	job, err := a.engine.Trigger(ctx, kind, taskLabels)
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

// Get implements ports.JobReader.
func (a *Adapter) Get(ctx context.Context, id string) (*pkgjobqueue.Job, error) {
	job, err := a.engine.Get(ctx, id)
	if err != nil {
		if errors.Is(err, pkgjobqueue.ErrNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	return job, nil
}
