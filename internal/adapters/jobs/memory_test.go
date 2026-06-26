package jobs

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

func waitForTerminal(t *testing.T, q ports.JobQueue, id string) *domain.Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := q.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if job.IsTerminal() {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach a terminal state within 2s", id)
	return nil
}

func TestQueue_SubmitCompletes(t *testing.T) {
	q := New(1)
	defer q.Close()

	submitted, err := q.Submit(context.Background(), "noop", nil, func(_ context.Context, _ ports.ProgressReporter) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submitted.Status != domain.JobStatusQueued {
		t.Errorf("initial Status = %s, want queued", submitted.Status)
	}

	got := waitForTerminal(t, q, submitted.ID)
	if got.Status != domain.JobStatusCompleted {
		t.Errorf("terminal Status = %s, want completed", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt is nil after completion")
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt is nil after completion")
	}
}

func TestQueue_SubmitFails(t *testing.T) {
	q := New(1)
	defer q.Close()

	boom := errors.New("something broke")
	submitted, _ := q.Submit(context.Background(), "failing", nil, func(_ context.Context, _ ports.ProgressReporter) error {
		return boom
	})

	got := waitForTerminal(t, q, submitted.ID)
	if got.Status != domain.JobStatusFailed {
		t.Errorf("Status = %s, want failed", got.Status)
	}
	if got.Error != boom.Error() {
		t.Errorf("Error = %q, want %q", got.Error, boom.Error())
	}
}

func TestQueue_Cancel(t *testing.T) {
	q := New(1)
	defer q.Close()

	started := make(chan struct{})
	submitted, _ := q.Submit(context.Background(), "blocking", nil, func(ctx context.Context, _ ports.ProgressReporter) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	<-started
	if err := q.Cancel(context.Background(), submitted.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got := waitForTerminal(t, q, submitted.ID)
	if got.Status != domain.JobStatusCancelled {
		t.Errorf("Status = %s, want cancelled", got.Status)
	}
}

func TestQueue_ProgressReport(t *testing.T) {
	q := New(1)
	defer q.Close()

	reported := make(chan struct{})
	submitted, _ := q.Submit(context.Background(), "progress", nil, func(_ context.Context, p ports.ProgressReporter) error {
		p.Report(5, 10, "halfway")
		close(reported)
		return nil
	})

	<-reported
	waitForTerminal(t, q, submitted.ID)

	got, _ := q.Get(context.Background(), submitted.ID)
	if got.Current != 5 || got.Total != 10 {
		t.Errorf("progress = %d/%d, want 5/10", got.Current, got.Total)
	}
	if got.Message != "halfway" {
		t.Errorf("Message = %q, want \"halfway\"", got.Message)
	}
}

func TestQueue_List(t *testing.T) {
	q := New(2)
	defer q.Close()

	for range 3 {
		q.Submit(context.Background(), "item", nil, func(_ context.Context, _ ports.ProgressReporter) error { //nolint:errcheck
			return nil
		})
	}

	// Drain all jobs before checking list length.
	jobs, err := q.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("List len = %d, want 3", len(jobs))
	}

	// Wait for all to finish and verify ordering (most recent first).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		all, _ := q.List(context.Background())
		done := true
		for _, j := range all {
			if !j.IsTerminal() {
				done = false
				break
			}
		}
		if done {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	all, _ := q.List(context.Background())
	for i := 1; i < len(all); i++ {
		if all[i].CreatedAt.After(all[i-1].CreatedAt) {
			t.Errorf("List not sorted most-recent-first at index %d", i)
		}
	}
}

func TestQueue_Get_NotFound(t *testing.T) {
	q := New(1)
	defer q.Close()

	_, err := q.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Get with unknown id returned nil error, want error")
	}
}

func TestQueue_Cancel_NotFound(t *testing.T) {
	q := New(1)
	defer q.Close()

	err := q.Cancel(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("Cancel with unknown id returned nil error, want error")
	}
}

func TestQueue_Cancel_QueuedJob(t *testing.T) {
	// Fill the worker so the second job stays queued.
	q := New(1)
	defer q.Close()

	blocking := make(chan struct{})
	q.Submit(context.Background(), "blocker", nil, func(_ context.Context, _ ports.ProgressReporter) error { //nolint:errcheck
		<-blocking
		return nil
	})

	queued, err := q.Submit(context.Background(), "queued", nil, func(_ context.Context, _ ports.ProgressReporter) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if err := q.Cancel(context.Background(), queued.ID); err != nil {
		t.Fatalf("Cancel queued: %v", err)
	}

	// Unblock the worker so the queue drains cleanly.
	close(blocking)

	job := waitForTerminal(t, q, queued.ID)
	if job.Status != domain.JobStatusCancelled {
		t.Errorf("Status = %s, want cancelled", job.Status)
	}
}

func TestQueue_Submit_WithPayload(t *testing.T) {
	q := New(1)
	defer q.Close()

	payload := map[string]any{"key": "value", "count": 42}
	submitted, err := q.Submit(context.Background(), "with-payload", payload, func(_ context.Context, _ ports.ProgressReporter) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got := waitForTerminal(t, q, submitted.ID)
	if got.Status != domain.JobStatusCompleted {
		t.Errorf("Status = %s, want completed", got.Status)
	}
	if got.Payload == nil {
		t.Fatal("Payload should not be nil")
	}
	if got.Payload["key"] != "value" {
		t.Errorf("Payload[key] = %v, want value", got.Payload["key"])
	}
}

func TestQueue_Submit_CancelledContext(t *testing.T) {
	q := New(1)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := q.Submit(ctx, "noop", nil, func(_ context.Context, _ ports.ProgressReporter) error {
		return nil
	})
	if err == nil {
		t.Error("Submit with cancelled context returned nil error, want error")
	}
}
