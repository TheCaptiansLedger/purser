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

	id, err := a.Trigger(ctx, "diagnostic", []string{"one"}, nil)
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
	_, err := a.Trigger(context.Background(), "nonexistent", nil, nil)
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

func TestAdapter_List(t *testing.T) {
	a := newAdapter()
	ctx := context.Background()

	id, err := a.Trigger(ctx, "diagnostic", []string{"one"}, nil)
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
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	page, _, err := a.List(ctx, "diagnostic", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	found := false
	for _, j := range page {
		if j.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("List(kind=diagnostic) did not include triggered job %q", id)
	}

	page, _, err = a.List(ctx, "nonexistent-kind", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(page) != 0 {
		t.Fatalf("List(kind=nonexistent-kind) returned %d jobs, want 0", len(page))
	}
}

func TestAdapter_Watch(t *testing.T) {
	a := newAdapter()
	ctx := context.Background()

	id, err := a.Trigger(ctx, "diagnostic", []string{"one"}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	events, unsubscribe, err := a.Watch(ctx, id)
	if err != nil {
		t.Fatalf("Watch returned error: %v", err)
	}
	defer unsubscribe()

	var last *pkgjobqueue.Event
	deadline := time.After(2 * time.Second)
loop:
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				break loop
			}
			if evt.Job.ID != id {
				t.Fatalf("event job id = %q, want %q", evt.Job.ID, id)
			}
			last = evt
		case <-deadline:
			t.Fatal("Watch did not close within the deadline")
		}
	}

	// Checked outside the select/for above (rather than in the channel-close
	// branch itself) so staticcheck's terminating-call analysis for t.Fatal
	// can actually follow the nil guard — nested inside a select case, SA5011
	// loses track of it and flags last.Job.Status below as a possible nil
	// dereference even though t.Fatal never returns. The explicit return
	// (redundant with t.Fatal's own runtime.Goexit) is what SA5011's
	// pattern-matching actually keys off, not just the terminating call.
	if last == nil {
		t.Fatal("Watch channel closed with no events delivered")
		return
	}
	if last.Job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("last event's job status = %q, want %q", last.Job.Status, pkgjobqueue.StatusSucceeded)
	}
}

func TestAdapter_Watch_NotFound(t *testing.T) {
	a := newAdapter()
	_, _, err := a.Watch(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Watch on unknown id returned %v, want ports.ErrNotFound", err)
	}
}

func TestAdapter_Get_WaitsForCompletion(t *testing.T) {
	a := newAdapter()
	ctx := context.Background()

	id, err := a.Trigger(ctx, "diagnostic", []string{"one", "two"}, nil)
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
