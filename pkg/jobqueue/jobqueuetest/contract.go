// Package jobqueuetest is the shared contract test suite for the
// jobqueue.Store port (see ADR 0003's contract-test convention, and
// pkg/cache/cachetest/pkg/fswatch/fswatchtest for the pattern this
// mirrors). It is a normal buildable package, not a _test.go file, because
// Go test files cannot be imported across packages — every adapter
// (pkg/jobqueue/memory today, others later) imports this from its own test
// file and runs it against its own constructor.
package jobqueuetest

import (
	"context"
	"errors"
	"purser/pkg/jobqueue"
	"testing"
	"time"
)

// NewStoreFunc returns a fresh, empty Store for the duration of a single
// subtest.
type NewStoreFunc func(t *testing.T) jobqueue.Store

// TestStore runs the shared Store contract against newStore. Each check is
// its own top-level subtest so a single failure identifies exactly which
// part of the contract broke.
func TestStore(t *testing.T, newStore NewStoreFunc) {
	t.Helper()

	t.Run("get on empty store misses", func(t *testing.T) { testGetOnEmptyMisses(t, newStore) })
	t.Run("create then get round-trips the job", func(t *testing.T) { testCreateThenGet(t, newStore) })
	t.Run("get returns a copy the caller can't mutate", func(t *testing.T) { testGetReturnsCopy(t, newStore) })
	t.Run("create stores a copy, not the caller's pointer", func(t *testing.T) { testCreateStoresCopy(t, newStore) })
	t.Run("update persists changes visible to a later get", func(t *testing.T) { testUpdatePersists(t, newStore) })
	t.Run("update on unknown id is not found", func(t *testing.T) { testUpdateUnknown(t, newStore) })
	t.Run("list paginates in creation order", func(t *testing.T) { testListPaginates(t, newStore) })
	t.Run("list filters by kind", func(t *testing.T) { testListFiltersByKind(t, newStore) })
	t.Run("list filters by status", func(t *testing.T) { testListFiltersByStatus(t, newStore) })
}

func newTestJob(id, kind string) *jobqueue.Job {
	return &jobqueue.Job{
		ID:        id,
		Kind:      kind,
		Status:    jobqueue.StatusPending,
		CreatedAt: time.Now(),
		Tasks: []*jobqueue.Task{
			{ID: id + "-task-1", Label: "task one", Status: jobqueue.StatusPending, Steps: []*jobqueue.Step{
				{ID: id + "-step-1", Name: "step one", Status: jobqueue.StatusPending, Detail: map[string]string{"k": "v"}},
			}},
		},
	}
}

func testGetOnEmptyMisses(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	_, err := s.GetJob(context.Background(), "missing")
	if !errors.Is(err, jobqueue.ErrNotFound) {
		t.Fatalf("GetJob on empty store returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	ctx := context.Background()

	job := newTestJob("job-1", "diagnostic")
	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Kind != "diagnostic" {
		t.Fatalf("GetJob.Kind = %q, want %q", got.Kind, "diagnostic")
	}
	if len(got.Tasks) != 1 || len(got.Tasks[0].Steps) != 1 {
		t.Fatalf("GetJob did not round-trip nested Tasks/Steps: %+v", got)
	}
	if got.Tasks[0].Steps[0].Detail["k"] != "v" {
		t.Fatalf("GetJob did not round-trip Step.Detail: %+v", got.Tasks[0].Steps[0])
	}
}

func testGetReturnsCopy(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	ctx := context.Background()

	job := newTestJob("job-1", "diagnostic")
	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	got.Kind = "mutated"
	got.Tasks[0].Label = "mutated"

	got2, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("second GetJob returned error: %v", err)
	}
	if got2.Kind == "mutated" || got2.Tasks[0].Label == "mutated" {
		t.Fatal("mutating a GetJob result leaked into the store")
	}
}

func testCreateStoresCopy(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	ctx := context.Background()

	job := newTestJob("job-1", "diagnostic")
	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	job.Kind = "mutated-after-create"

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Kind == "mutated-after-create" {
		t.Fatal("mutating the pointer passed to CreateJob leaked into the store")
	}
}

func testUpdatePersists(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	ctx := context.Background()

	job := newTestJob("job-1", "diagnostic")
	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}

	job.Status = jobqueue.StatusRunning
	job.Tasks[0].Status = jobqueue.StatusRunning
	if err := s.UpdateJob(ctx, job); err != nil {
		t.Fatalf("UpdateJob returned error: %v", err)
	}

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Status != jobqueue.StatusRunning {
		t.Fatalf("GetJob.Status = %q after update, want %q", got.Status, jobqueue.StatusRunning)
	}
	if got.Tasks[0].Status != jobqueue.StatusRunning {
		t.Fatalf("GetJob.Tasks[0].Status = %q after update, want %q", got.Tasks[0].Status, jobqueue.StatusRunning)
	}
}

func testUpdateUnknown(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	job := newTestJob("missing", "diagnostic")
	if err := s.UpdateJob(context.Background(), job); !errors.Is(err, jobqueue.ErrNotFound) {
		t.Fatalf("UpdateJob on unknown id returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	ctx := context.Background()

	const n = 5
	for i := 0; i < n; i++ {
		job := newTestJob(string(rune('a'+i)), "diagnostic")
		job.CreatedAt = time.Now()
		if err := s.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob(%d) returned error: %v", i, err)
		}
	}

	seen := map[string]bool{}
	token := ""
	for {
		page, next, err := s.ListJobs(ctx, "", "", 2, token)
		if err != nil {
			t.Fatalf("ListJobs returned error: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, j := range page {
			if seen[j.ID] {
				t.Fatalf("ListJobs returned job %q twice across pages", j.ID)
			}
			seen[j.ID] = true
		}
		if next == "" {
			break
		}
		token = next
	}

	if len(seen) != n {
		t.Fatalf("ListJobs paginated through %d jobs, want %d", len(seen), n)
	}
}

func testListFiltersByKind(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	ctx := context.Background()

	for i, kind := range []string{"diagnostic", "scan", "diagnostic", "scan", "diagnostic"} {
		job := newTestJob(string(rune('a'+i)), kind)
		job.CreatedAt = time.Now()
		if err := s.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob(%d) returned error: %v", i, err)
		}
	}

	page, next, err := s.ListJobs(ctx, "scan", "", 10, "")
	if err != nil {
		t.Fatalf("ListJobs returned error: %v", err)
	}
	if next != "" {
		t.Fatalf("ListJobs next_page_token = %q, want empty (only 2 matching jobs, page_size 10)", next)
	}
	if len(page) != 2 {
		t.Fatalf("ListJobs(kind=scan) returned %d jobs, want 2", len(page))
	}
	for _, j := range page {
		if j.Kind != "scan" {
			t.Fatalf("ListJobs(kind=scan) returned a job with kind %q", j.Kind)
		}
	}
}

func testListFiltersByStatus(t *testing.T, newStore NewStoreFunc) {
	s := newStore(t)
	ctx := context.Background()

	statuses := []jobqueue.Status{jobqueue.StatusSucceeded, jobqueue.StatusFailed, jobqueue.StatusPartial, jobqueue.StatusSucceeded}
	for i, status := range statuses {
		job := newTestJob(string(rune('a'+i)), "diagnostic")
		job.CreatedAt = time.Now()
		job.Status = status
		if err := s.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob(%d) returned error: %v", i, err)
		}
	}

	page, next, err := s.ListJobs(ctx, "", jobqueue.StatusPartial, 10, "")
	if err != nil {
		t.Fatalf("ListJobs returned error: %v", err)
	}
	if next != "" {
		t.Fatalf("ListJobs next_page_token = %q, want empty (only 1 matching job, page_size 10)", next)
	}
	if len(page) != 1 {
		t.Fatalf("ListJobs(status=partial) returned %d jobs, want 1", len(page))
	}
	if page[0].Status != jobqueue.StatusPartial {
		t.Fatalf("ListJobs(status=partial) returned a job with status %q", page[0].Status)
	}
}
