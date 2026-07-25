package jobqueue

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Runner is what an Executor uses to report progress on one Job. Every
// method both persists the corresponding change to the Engine's Store
// (what a caller polling GetJob reads) and, for Step transitions, emits
// the accompanying span/metric/log line — the same event serving both
// audiences, never one instead of the other. See
// docs/adr/0023-job-queue.md.
type Runner struct {
	engine *Engine
	jobID  string
}

// Job returns the current full state of the Job being run.
func (r *Runner) Job(ctx context.Context) (*Job, error) {
	return r.engine.store.GetJob(ctx, r.jobID)
}

// mutateTask loads the Job, applies fn to the named Task, and persists the
// result.
func (r *Runner) mutateTask(ctx context.Context, taskID string, fn func(*Task)) error {
	job, err := r.engine.store.GetJob(ctx, r.jobID)
	if err != nil {
		return err
	}
	t := findTask(job, taskID)
	if t == nil {
		return fmt.Errorf("jobqueue: task %q not found in job %q", taskID, r.jobID)
	}
	fn(t)
	return r.engine.store.UpdateJob(ctx, job)
}

// StartTask marks taskID as running.
func (r *Runner) StartTask(ctx context.Context, taskID string) error {
	return r.mutateTask(ctx, taskID, func(t *Task) {
		t.Status = StatusRunning
		t.StartedAt = time.Now()
	})
}

// FinishTask marks taskID with its terminal status.
func (r *Runner) FinishTask(ctx context.Context, taskID string, status Status) error {
	return r.mutateTask(ctx, taskID, func(t *Task) {
		t.Status = status
		t.FinishedAt = time.Now()
	})
}

// StepHandle is returned by StartStep and passed to FinishStep — it carries
// the step's identity and its open span/start time across the two calls.
type StepHandle struct {
	ctx     context.Context
	span    trace.Span
	taskID  string
	stepID  string
	name    string
	started time.Time
}

// StartStep appends a new running Step named name to taskID, persists it,
// and opens the span that FinishStep later closes.
func (r *Runner) StartStep(ctx context.Context, taskID, name string) (*StepHandle, error) {
	stepID := newID()
	started := time.Now()
	err := r.mutateTask(ctx, taskID, func(t *Task) {
		t.Steps = append(t.Steps, &Step{ID: stepID, Name: name, Status: StatusRunning, StartedAt: started})
	})
	if err != nil {
		return nil, err
	}

	stepCtx, span := r.engine.tracer.Start(ctx, "jobqueue.step", trace.WithAttributes(
		attribute.String("job.id", r.jobID),
		attribute.String("task.id", taskID),
		attribute.String("step.name", name),
	))
	r.engine.logger.InfoContext(stepCtx, "step started",
		"job.id", r.jobID, "task.id", taskID, "step.name", name,
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
	)

	return &StepHandle{ctx: stepCtx, span: span, taskID: taskID, stepID: stepID, name: name, started: started}, nil
}

// FinishStep records h's step as finished with status/message/detail,
// closes its span, and records the step's duration metric and log line.
// err is the underlying cause when status is StatusFailed (nil otherwise):
// it is never folded into message, only passed as a structured slog
// attribute per docs/adr/0008-structured-logging.md, and its presence is
// what selects Error- over Info-level logging for the "step finished"
// event — success and failure are the same event, not two logging paths.
func (r *Runner) FinishStep(h *StepHandle, status Status, message string, detail map[string]string, err error) error {
	finished := time.Now()
	mutateErr := r.mutateTask(h.ctx, h.taskID, func(t *Task) {
		s := findStep(t, h.stepID)
		if s == nil {
			return
		}
		s.Status = status
		s.FinishedAt = finished
		s.Message = message
		s.Detail = detail
	})

	elapsed := finished.Sub(h.started)
	attrs := metric.WithAttributes(
		attribute.String("step.name", h.name),
		attribute.String("step.status", string(status)),
	)
	r.engine.stepDuration.Record(h.ctx, elapsed.Seconds(), attrs)
	r.engine.stepsTotal.Add(h.ctx, 1, attrs)

	h.span.SetAttributes(attribute.String("step.status", string(status)))
	h.span.End()

	logArgs := []any{
		"job.id", r.jobID, "task.id", h.taskID, "step.name", h.name, "status", string(status),
		"elapsed_ms", elapsed.Milliseconds(),
		"trace_id", h.span.SpanContext().TraceID().String(),
		"span_id", h.span.SpanContext().SpanID().String(),
	}
	if err != nil {
		r.engine.logger.ErrorContext(h.ctx, "step finished", append(logArgs, "error", err)...)
	} else {
		r.engine.logger.InfoContext(h.ctx, "step finished", logArgs...)
	}

	return mutateErr
}
