package pipeline_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"purser/internal/adapters/pipeline"
	"purser/internal/domain"
	"purser/internal/ports"
	pkgjobqueue "purser/pkg/jobqueue"
	"purser/pkg/jobqueue/memory"
	"sync"
	"testing"
	"time"
)

// fakeUnmatchedFileRepository is a minimal in-memory
// ports.UnmatchedFileRepository double — ScanExecutor's storage behavior
// is already covered by internal/adapters/store/unmatchedfile's own
// contract test; this test's job is ScanExecutor's orchestration against
// the real pkgjobqueue.Runner/Engine, which can't be faked.
type fakeUnmatchedFileRepository struct {
	mu    sync.Mutex
	files map[string]*domain.UnmatchedFile
}

func newFakeUnmatchedFileRepository() *fakeUnmatchedFileRepository {
	return &fakeUnmatchedFileRepository{files: make(map[string]*domain.UnmatchedFile)}
}

func (f *fakeUnmatchedFileRepository) Create(_ context.Context, u *domain.UnmatchedFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.files[u.ID]; exists {
		return ports.ErrConflict
	}
	cp := *u
	f.files[u.ID] = &cp
	return nil
}

func (f *fakeUnmatchedFileRepository) Get(_ context.Context, id string) (*domain.UnmatchedFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.files[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func newEngine(t *testing.T, repo ports.UnmatchedFileRepository) *pkgjobqueue.Engine {
	t.Helper()
	engine := pkgjobqueue.NewEngine(memory.New())
	engine.Register("scan", pipeline.NewScanExecutor(repo))
	return engine
}

func waitForTerminal(t *testing.T, engine *pkgjobqueue.Engine, jobID string) *pkgjobqueue.Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := engine.Get(context.Background(), jobID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		switch job.Status {
		case pkgjobqueue.StatusSucceeded, pkgjobqueue.StatusFailed, pkgjobqueue.StatusPartial:
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not reach a terminal status in time")
	return nil
}

// writeHashableFixture writes a file large enough for OSHash (>= 64 KiB).
func writeHashableFixture(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, 70000)
	for i := range data {
		data[i] = byte(i)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	return path
}

func findStep(task *pkgjobqueue.Task, name string) *pkgjobqueue.Step {
	for _, s := range task.Steps {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func TestScanExecutor_Execute_Success(t *testing.T) {
	dir := t.TempDir()
	file1 := writeHashableFixture(t, dir, "one.flac")
	file2 := writeHashableFixture(t, dir, "two.flac")

	repo := newFakeUnmatchedFileRepository()
	engine := newEngine(t, repo)

	id, err := engine.Trigger(context.Background(), "scan", []string{file1, file2}, map[string]string{
		"enable_md5":    "true",
		"enable_sha512": "false",
	})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}
	if len(job.Tasks) != 2 {
		t.Fatalf("job has %d tasks, want 2", len(job.Tasks))
	}

	for _, task := range job.Tasks {
		if len(task.Steps) != 2 || task.Steps[0].Name != "hash" || task.Steps[1].Name != "queue" {
			t.Fatalf("task %q has steps %+v, want [hash queue]", task.Label, task.Steps)
		}

		hashStep := findStep(task, "hash")
		if hashStep.Detail["oshash"] == "" {
			t.Errorf("task %q hash step missing oshash", task.Label)
		}
		if hashStep.Detail["sha1"] == "" {
			t.Errorf("task %q hash step missing sha1", task.Label)
		}
		if hashStep.Detail["md5"] == "" {
			t.Errorf("task %q hash step missing md5 (enabled)", task.Label)
		}
		if _, hasSHA512 := hashStep.Detail["sha512"]; hasSHA512 {
			t.Errorf("task %q hash step has sha512 detail, want absent (disabled)", task.Label)
		}

		queueStep := findStep(task, "queue")
		ufID := queueStep.Detail["unmatched_file.id"]
		if ufID == "" {
			t.Fatalf("task %q queue step missing unmatched_file.id", task.Label)
		}

		got, err := repo.Get(context.Background(), ufID)
		if err != nil {
			t.Fatalf("repo.Get(%q) returned error: %v", ufID, err)
		}
		if got.Path != task.Label {
			t.Errorf("stored UnmatchedFile.Path = %q, want %q", got.Path, task.Label)
		}
		if got.OSHash != hashStep.Detail["oshash"] {
			t.Errorf("stored UnmatchedFile.OSHash = %q, want %q", got.OSHash, hashStep.Detail["oshash"])
		}
		if got.Status != domain.UnmatchedFileStatusPending {
			t.Errorf("stored UnmatchedFile.Status = %q, want %q", got.Status, domain.UnmatchedFileStatusPending)
		}
	}
}

func TestScanExecutor_Execute_SHA512Enabled(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	engine := newEngine(t, repo)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{
		"enable_md5":    "false",
		"enable_sha512": "true",
	})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	hashStep := findStep(job.Tasks[0], "hash")
	if hashStep.Detail["sha512"] == "" {
		t.Error("hash step missing sha512 (enabled)")
	}
	if _, hasMD5 := hashStep.Detail["md5"]; hasMD5 {
		t.Error("hash step has md5 detail, want absent (disabled)")
	}

	ufID := findStep(job.Tasks[0], "queue").Detail["unmatched_file.id"]
	got, err := repo.Get(context.Background(), ufID)
	if err != nil {
		t.Fatalf("repo.Get returned error: %v", err)
	}
	if got.SHA512 == "" {
		t.Error("stored UnmatchedFile.SHA512 is empty, want populated")
	}
	if got.MD5 != "" {
		t.Errorf("stored UnmatchedFile.MD5 = %q, want empty (disabled)", got.MD5)
	}
}

func TestScanExecutor_Execute_NoParamsDefaultsBothHashesOff(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	engine := newEngine(t, repo)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	hashStep := findStep(job.Tasks[0], "hash")
	if _, has := hashStep.Detail["md5"]; has {
		t.Error("hash step has md5 detail, want absent when params is nil")
	}
	if _, has := hashStep.Detail["sha512"]; has {
		t.Error("hash step has sha512 detail, want absent when params is nil")
	}
}

func TestScanExecutor_Execute_HashFailureSkipsQueueStep(t *testing.T) {
	repo := newFakeUnmatchedFileRepository()
	engine := newEngine(t, repo)

	missing := filepath.Join(t.TempDir(), "does-not-exist.flac")
	id, err := engine.Trigger(context.Background(), "scan", []string{missing}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusFailed)
	}
	if len(job.Tasks) != 1 {
		t.Fatalf("job has %d tasks, want 1", len(job.Tasks))
	}

	task := job.Tasks[0]
	if len(task.Steps) != 1 || task.Steps[0].Name != "hash" {
		t.Fatalf("task has steps %+v, want exactly [hash]", task.Steps)
	}
	if task.Steps[0].Status != pkgjobqueue.StatusFailed {
		t.Fatalf("hash step status = %q, want %q", task.Steps[0].Status, pkgjobqueue.StatusFailed)
	}
	if len(repo.files) != 0 {
		t.Fatalf("repo has %d files, want 0 (queue step must never have run)", len(repo.files))
	}
}

func TestScanExecutor_Execute_RepositoryErrorFailsTask(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	failingRepo := &alwaysFailRepository{err: errors.New("boom")}
	failEngine := pkgjobqueue.NewEngine(memory.New())
	failEngine.Register("scan", pipeline.NewScanExecutor(failingRepo))

	id, err := failEngine.Trigger(context.Background(), "scan", []string{file}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, failEngine, id.ID)
	if job.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusFailed)
	}
	queueStep := findStep(job.Tasks[0], "queue")
	if queueStep == nil || queueStep.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("queue step = %+v, want a failed step", queueStep)
	}
}

type alwaysFailRepository struct {
	err error
}

func (a *alwaysFailRepository) Create(context.Context, *domain.UnmatchedFile) error {
	return a.err
}

func (a *alwaysFailRepository) Get(context.Context, string) (*domain.UnmatchedFile, error) {
	return nil, ports.ErrNotFound
}
