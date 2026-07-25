package jobqueue_test

import (
	"context"
	"errors"
	"purser/internal/adapters/jobqueue"
	"purser/internal/ports"
	pkgjobqueue "purser/pkg/jobqueue"
	"purser/pkg/jobqueue/memory"
	"testing"
	"time"
)

func newAdapter() *jobqueue.Adapter {
	engine := pkgjobqueue.NewEngine(memory.New())
	return jobqueue.New(engine)
}

func TestAdapter_TriggerAndGet(t *testing.T) {
	a := newAdapter()
	ctx := context.Background()

	id, err := a.Trigger(ctx, "diagnostic", []string{"one"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if id == "" {
		t.Fatal("Trigger returned an empty job id")
	}

	job, err := a.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if job.ID != id {
		t.Fatalf("Get returned job with ID %q, want %q", job.ID, id)
	}
}

func TestAdapter_Trigger_UnknownKind(t *testing.T) {
	a := newAdapter()
	_, err := a.Trigger(context.Background(), "nonexistent", nil)
	if !errors.Is(err, pkgjobqueue.ErrUnknownKind) {
		t.Fatalf("Trigger with unknown kind returned %v, want ErrUnknownKind", err)
	}
}

func TestAdapter_Get_NotFound(t *testing.T) {
	a := newAdapter()
	_, err := a.Get(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on unknown id returned %v, want ports.ErrNotFound", err)
	}
}

func TestAdapter_Get_WaitsForCompletion(t *testing.T) {
	a := newAdapter()
	ctx := context.Background()

	id, err := a.Trigger(ctx, "diagnostic", []string{"one", "two"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := a.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if job.Status == pkgjobqueue.StatusSucceeded {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not reach succeeded status in time")
}
