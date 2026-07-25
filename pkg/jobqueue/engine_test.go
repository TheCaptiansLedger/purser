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

func TestEngine_List(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	job, err := eng.Trigger(context.Background(), "diagnostic", []string{"one"}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	waitForTerminal(t, eng, job.ID)

	page, _, err := eng.List(context.Background(), "diagnostic", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	found := false
	for _, j := range page {
		if j.ID == job.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("List(kind=diagnostic) did not include triggered job %q", job.ID)
	}

	page, _, err = eng.List(context.Background(), "nonexistent-kind", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page) != 0 {
		t.Fatalf("List(kind=nonexistent-kind) returned %d jobs, want 0", len(page))
	}
}

func TestEngine_Watch_NotFound(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))
	_, _, err := eng.Watch(context.Background(), "missing")
	if !errors.Is(err, jobqueue.ErrNotFound) {
		t.Fatalf("Watch on unknown id returned %v, want ErrNotFound", err)
	}
}

func TestEngine_Watch_DeliversEventsAndClosesOnTerminal(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	job, err := eng.Trigger(context.Background(), "diagnostic", []string{"one"}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	events, unsubscribe, err := eng.Watch(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Watch returned error: %v", err)
	}
	defer unsubscribe()

	var got []*jobqueue.Event
	deadline := time.After(2 * time.Second)
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				final := waitForTerminal(t, eng, job.ID)
				if len(got) == 0 {
					t.Fatal("Watch closed without delivering any events")
				}
				last := got[len(got)-1]
				if last.Job.Status != final.Status {
					t.Fatalf("last delivered event's job status = %q, want %q (matching GetJob at completion)", last.Job.Status, final.Status)
				}
				return
			}
			if evt.Job.ID != job.ID {
				t.Fatalf("event job id = %q, want %q", evt.Job.ID, job.ID)
			}
			got = append(got, evt)
		case <-deadline:
			t.Fatal("Watch did not close within the deadline")
		}
	}
}

func TestEngine_Watch_UnsubscribeStopsFurtherDelivery(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	exec := &blockingExecutor{started: make(chan struct{}), proceed: make(chan struct{}), taskStatus: jobqueue.StatusSucceeded}
	eng.Register("watch-unsub", exec)

	job, err := eng.Trigger(context.Background(), "watch-unsub", []string{"only task"}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	select {
	case <-exec.started:
	case <-time.After(2 * time.Second):
		t.Fatal("executor never started")
	}

	events, unsubscribe, err := eng.Watch(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Watch returned error: %v", err)
	}

	// Drain the priming snapshot event, then unsubscribe before the Job
	// finishes — no further event (in particular, the terminal one) must
	// ever arrive on this channel.
	<-events
	unsubscribe()

	close(exec.proceed)
	waitForTerminal(t, eng, job.ID)

	select {
	case evt, ok := <-events:
		if ok {
			t.Fatalf("received an event after unsubscribe: %v", evt)
		}
		// A closed channel with no further sends is exactly what an
		// unsubscribed, already-drained channel looks like.
	default:
		t.Fatal("channel should be closed after unsubscribe, not still open with nothing pending")
	}
}

func TestEngine_Watch_PrimingEventReflectsCurrentState(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	job, err := eng.Trigger(context.Background(), "diagnostic", []string{"one"}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	waitForTerminal(t, eng, job.ID)

	// Watching a Job that already finished before Watch was ever called
	// must still deliver one event (the current, terminal state) rather
	// than hanging forever waiting for a transition that already happened.
	events, unsubscribe, err := eng.Watch(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Watch returned error: %v", err)
	}
	defer unsubscribe()

	select {
	case evt, ok := <-events:
		if !ok {
			t.Fatal("channel closed with no priming event for an already-finished Job")
		}
		if evt.Job.Status != jobqueue.StatusSucceeded {
			t.Fatalf("priming event status = %q, want %q", evt.Job.Status, jobqueue.StatusSucceeded)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not deliver the priming event in time")
	}

	select {
	case evt, ok := <-events:
		if ok {
			t.Fatalf("received a second event for an already-finished Job: %v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after the priming event for an already-finished Job")
	}
}

// executorFunc adapts a plain function to jobqueue.Executor.
type executorFunc func(ctx context.Context, r *jobqueue.Runner) error

func (f executorFunc) Execute(ctx context.Context, r *jobqueue.Runner) error { return f(ctx, r) }
