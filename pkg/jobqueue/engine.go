package jobqueue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/pkg/jobqueue"

// Executor performs the work for one Job of a given Kind. Implementations
// report progress by calling back into the Runner passed to Execute — the
// Runner is what actually mutates and persists Job/Task/Step state, and
// emits the accompanying telemetry/log event, so an Executor never touches
// the Store directly.
type Executor interface {
	Execute(ctx context.Context, r *Runner) error
}

// Engine creates and drives Jobs against a Store, dispatching each Job's
// work to the Executor registered for its Kind. See docs/adr/0023-job-queue.md.
type Engine struct {
	store Store

	mu        sync.RWMutex
	executors map[string]Executor

	logger *slog.Logger
	tracer trace.Tracer

	stepDuration metric.Float64Histogram
	stepsTotal   metric.Int64Counter
	jobsTotal    metric.Int64Counter
}

// NewEngine constructs an Engine backed by store, with the built-in
// "diagnostic" Executor already registered — a permanent smoke-testing
// capability of the engine, not scaffolding a caller wires up. Additional
// Executors are added via Register.
func NewEngine(store Store, opts ...Option) *Engine {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	e := &Engine{
		store:     store,
		executors: make(map[string]Executor),
		logger:    o.logger.With("component", "jobqueue"),
		tracer:    o.tracerProvider.Tracer(instrumentationName),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if e.stepDuration, err = meter.Float64Histogram("jobqueue.step.duration",
		metric.WithDescription("duration of a single job/task step, in seconds"),
		metric.WithUnit("s"),
	); err != nil {
		panic("jobqueue: creating step duration histogram: " + err.Error())
	}
	if e.stepsTotal, err = meter.Int64Counter("jobqueue.steps",
		metric.WithDescription("job/task steps completed"),
	); err != nil {
		panic("jobqueue: creating steps counter: " + err.Error())
	}
	if e.jobsTotal, err = meter.Int64Counter("jobqueue.jobs",
		metric.WithDescription("jobs triggered"),
	); err != nil {
		panic("jobqueue: creating jobs counter: " + err.Error())
	}

	e.Register("diagnostic", newDiagnosticExecutor())
	return e
}

// Register adds (or replaces) the Executor used for Jobs of the given
// Kind.
func (e *Engine) Register(kind string, exec Executor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executors[kind] = exec
}

func (e *Engine) executor(kind string) (Executor, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	exec, ok := e.executors[kind]
	return exec, ok
}

// Trigger creates a new Job of the given kind with one pending Task per
// label, persists it, and starts execution asynchronously in a detached
// goroutine — the returned Job reflects only its initial (pending) state,
// callers poll Get for progress. params is opaque, kind-specific
// configuration the engine passes through unexamined — only the registered
// Executor for kind interprets its keys. Returns ErrUnknownKind if kind has
// no registered Executor.
func (e *Engine) Trigger(ctx context.Context, kind string, taskLabels []string, params map[string]string) (*Job, error) {
	exec, ok := e.executor(kind)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}

	now := time.Now()
	job := &Job{
		ID:        newID(),
		Kind:      kind,
		Status:    StatusPending,
		CreatedAt: now,
		Params:    params,
	}
	for _, label := range taskLabels {
		job.Tasks = append(job.Tasks, &Task{ID: newID(), Label: label, Status: StatusPending})
	}

	if err := e.store.CreateJob(ctx, job); err != nil {
		return nil, err
	}

	e.jobsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("job.kind", kind)))
	e.logger.InfoContext(ctx, "job triggered", "job.id", job.ID, "job.kind", kind, "task_count", len(job.Tasks))

	// Detached from ctx's cancellation (the RPC that triggered this
	// returns immediately) but keeps ctx's values, including the active
	// span, so the background run still correlates to the triggering
	// trace.
	runCtx := context.WithoutCancel(ctx)
	go e.run(runCtx, job.ID, exec)

	return job.Clone(), nil
}

// Get returns the current full state of the Job with the given id, or
// ErrNotFound.
func (e *Engine) Get(ctx context.Context, id string) (*Job, error) {
	return e.store.GetJob(ctx, id)
}

// run drives a single Job's Executor to completion and records the final
// Status. It runs in its own goroutine, started by Trigger.
func (e *Engine) run(ctx context.Context, jobID string, exec Executor) {
	ctx, span := e.tracer.Start(ctx, "jobqueue.run", trace.WithAttributes(attribute.String("job.id", jobID)))
	defer span.End()

	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		e.logger.ErrorContext(ctx, "job disappeared before running", "job.id", jobID, "error", err)
		return
	}

	job.Status = StatusRunning
	job.StartedAt = time.Now()
	if err := e.store.UpdateJob(ctx, job); err != nil {
		e.logger.ErrorContext(ctx, "recording job running", "job.id", jobID, "error", err)
		return
	}

	execErr := exec.Execute(ctx, &Runner{engine: e, jobID: jobID})
	if execErr != nil {
		e.logger.ErrorContext(ctx, "job executor returned an error", "job.id", jobID, "error", execErr)
	}

	job, err = e.store.GetJob(ctx, jobID)
	if err != nil {
		e.logger.ErrorContext(ctx, "job disappeared after running", "job.id", jobID, "error", err)
		return
	}
	job.FinishedAt = time.Now()
	job.Status = finalStatus(job, execErr)
	if err := e.store.UpdateJob(ctx, job); err != nil {
		e.logger.ErrorContext(ctx, "recording job finished", "job.id", jobID, "error", err)
		return
	}

	e.logger.InfoContext(ctx, "job finished", "job.id", jobID, "status", string(job.Status))
}

// finalStatus derives a Job's terminal Status from its Tasks' outcomes and
// whether the Executor itself returned an error.
func finalStatus(job *Job, execErr error) Status {
	if execErr != nil {
		return StatusFailed
	}
	if len(job.Tasks) == 0 {
		return StatusSucceeded
	}
	succeeded, failed := 0, 0
	for _, t := range job.Tasks {
		switch t.Status {
		case StatusSucceeded:
			succeeded++
		case StatusFailed:
			failed++
		}
	}
	switch {
	case failed == 0:
		return StatusSucceeded
	case succeeded == 0:
		return StatusFailed
	default:
		return StatusPartial
	}
}

// Option customizes an Engine constructed via NewEngine.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
}

func defaultOptions() *options {
	return &options{
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
	}
}

// WithLogger overrides the default (slog.Default()) logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithTracerProvider overrides the default (global) TracerProvider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) { o.tracerProvider = tp }
}

// WithMeterProvider overrides the default (global) MeterProvider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(o *options) { o.meterProvider = mp }
}
