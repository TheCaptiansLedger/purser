package jobqueue_test

import (
	"context"
	"errors"
	"log/slog"
	"purser/pkg/jobqueue"
	"purser/pkg/jobqueue/memory"
	"testing"
	"time"
)

// blockingExecutor marks its single task running, signals started, waits
// for proceed to close, then finishes the task with the given status.
type blockingExecutor struct {
	started    chan struct{}
	proceed    chan struct{}
	taskStatus jobqueue.Status
}

func (e *blockingExecutor) Execute(ctx context.Context, r *jobqueue.Runner) error {
	job, err := r.Job(ctx)
	if err != nil {
		return err
	}
	if err := r.StartTask(ctx, job.Tasks[0].ID); err != nil {
		return err
	}
	close(e.started)
	<-e.proceed
	return r.FinishTask(ctx, job.Tasks[0].ID, e.taskStatus)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func waitForTerminal(t *testing.T, eng *jobqueue.Engine, id string) *jobqueue.Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := eng.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if job.Status == jobqueue.StatusSucceeded || job.Status == jobqueue.StatusFailed || job.Status == jobqueue.StatusPartial {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %q did not reach a terminal status in time", id)
	return nil
}

func TestEngine_Trigger_UnknownKind(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))
	_, err := eng.Trigger(context.Background(), "nonexistent", []string{"a"}, nil)
	if !errors.Is(err, jobqueue.ErrUnknownKind) {
		t.Fatalf("Trigger with unknown kind returned %v, want ErrUnknownKind", err)
	}
}

func TestEngine_Trigger_Lifecycle(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	exec := &blockingExecutor{started: make(chan struct{}), proceed: make(chan struct{}), taskStatus: jobqueue.StatusSucceeded}
	eng.Register("block", exec)

	job, err := eng.Trigger(context.Background(), "block", []string{"only task"}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if job.ID == "" {
		t.Fatal("Trigger returned a job with no ID")
	}
	if len(job.Tasks) != 1 || job.Tasks[0].Label != "only task" {
		t.Fatalf("Trigger did not create the expected task: %+v", job.Tasks)
	}

	select {
	case <-exec.started:
	case <-time.After(2 * time.Second):
		t.Fatal("executor never started")
	}

	running, err := eng.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if running.Status != jobqueue.StatusRunning {
		t.Fatalf("Job.Status = %q while executor is blocked, want %q", running.Status, jobqueue.StatusRunning)
	}
	if running.Tasks[0].Status != jobqueue.StatusRunning {
		t.Fatalf("Task.Status = %q while executor is blocked, want %q", running.Tasks[0].Status, jobqueue.StatusRunning)
	}

	close(exec.proceed)

	final := waitForTerminal(t, eng, job.ID)
	if final.Status != jobqueue.StatusSucceeded {
		t.Fatalf("final Job.Status = %q, want %q", final.Status, jobqueue.StatusSucceeded)
	}
	if final.FinishedAt.IsZero() {
		t.Fatal("final Job.FinishedAt is zero")
	}
	if final.Tasks[0].Status != jobqueue.StatusSucceeded {
		t.Fatalf("final Task.Status = %q, want %q", final.Tasks[0].Status, jobqueue.StatusSucceeded)
	}
}

func TestEngine_Trigger_PartialOnMixedTaskOutcomes(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	eng.Register("mixed", executorFunc(func(ctx context.Context, r *jobqueue.Runner) error {
		job, err := r.Job(ctx)
		if err != nil {
			return err
		}
		if err := r.StartTask(ctx, job.Tasks[0].ID); err != nil {
			return err
		}
		if err := r.FinishTask(ctx, job.Tasks[0].ID, jobqueue.StatusSucceeded); err != nil {
			return err
		}
		if err := r.StartTask(ctx, job.Tasks[1].ID); err != nil {
			return err
		}
		return r.FinishTask(ctx, job.Tasks[1].ID, jobqueue.StatusFailed)
	}))

	job, err := eng.Trigger(context.Background(), "mixed", []string{"one", "two"}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	final := waitForTerminal(t, eng, job.ID)
	if final.Status != jobqueue.StatusPartial {
		t.Fatalf("final Job.Status = %q, want %q", final.Status, jobqueue.StatusPartial)
	}
}

// executorFunc adapts a plain function to jobqueue.Executor.
type executorFunc func(ctx context.Context, r *jobqueue.Runner) error

func (f executorFunc) Execute(ctx context.Context, r *jobqueue.Runner) error { return f(ctx, r) }
