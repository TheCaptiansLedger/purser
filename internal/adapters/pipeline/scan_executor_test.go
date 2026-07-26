package pipeline_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"purser/internal/adapters/pipeline"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/filehash"
	pkgjobqueue "purser/pkg/jobqueue"
	"purser/pkg/jobqueue/memory"
	"sync"
	"testing"
	"time"
)

// fakeUnmatchedFileRepository is a minimal in-memory
// ports.UnmatchedFileRepository double — storage behavior is already
// covered by internal/adapters/store/unmatchedfile's own contract test;
// this test's job is ScanExecutor's orchestration against the real
// pkgjobqueue.Runner/Engine, which can't be faked. GetByHash's ordered
// lookup is deliberately re-implemented (not shared with the store
// package) so it can be exercised without a real Datastore.
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

func (f *fakeUnmatchedFileRepository) Update(_ context.Context, u *domain.UnmatchedFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[u.ID]; !ok {
		return ports.ErrNotFound
	}
	cp := *u
	f.files[u.ID] = &cp
	return nil
}

// Delete is unused by ScanExecutor's tests (it only Creates/Gets/Updates)
// — present solely to satisfy ports.UnmatchedFileRepository.
func (f *fakeUnmatchedFileRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.files, id)
	return nil
}

// List is unused by ScanExecutor's tests (it only Creates/Gets/Updates) —
// present solely to satisfy ports.UnmatchedFileRepository.
func (f *fakeUnmatchedFileRepository) List(_ context.Context, status domain.UnmatchedFileStatus, _ int, _ string) ([]*domain.UnmatchedFile, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.UnmatchedFile
	for _, u := range f.files {
		if status == "" || u.Status == status {
			cp := *u
			out = append(out, &cp)
		}
	}
	return out, "", nil
}

// UpdateBatch is unused by ScanExecutor's tests (it only Creates/Gets/
// Updates) — present solely to satisfy ports.UnmatchedFileRepository.
func (f *fakeUnmatchedFileRepository) UpdateBatch(_ context.Context, us []*domain.UnmatchedFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range us {
		if _, ok := f.files[u.ID]; !ok {
			return ports.ErrNotFound
		}
	}
	for _, u := range us {
		cp := *u
		f.files[u.ID] = &cp
	}
	return nil
}

// ListByGroupKey is unused by ScanExecutor's tests (it only Creates/Gets/
// Updates) — present solely to satisfy ports.UnmatchedFileRepository.
func (f *fakeUnmatchedFileRepository) ListByGroupKey(_ context.Context, groupKey string) ([]*domain.UnmatchedFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.UnmatchedFile
	for _, u := range f.files {
		if u.GroupKey == groupKey {
			cp := *u
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeUnmatchedFileRepository) GetByHash(_ context.Context, oshash, sha1sum, md5sum, sha512sum string) (*domain.UnmatchedFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, want := range []struct{ field, value string }{
		{"oshash", oshash}, {"sha1", sha1sum}, {"md5", md5sum}, {"sha512", sha512sum},
	} {
		if want.value == "" {
			continue
		}
		for _, u := range f.files {
			var got string
			switch want.field {
			case "oshash":
				got = u.OSHash
			case "sha1":
				got = u.SHA1
			case "md5":
				got = u.MD5
			case "sha512":
				got = u.SHA512
			}
			if got == want.value {
				cp := *u
				return &cp, nil
			}
		}
	}
	return nil, ports.ErrNotFound
}

// fakeMediaFileRepository is a minimal in-memory ports.MediaFileRepository
// double, following the same convention as fakeUnmatchedFileRepository
// above.
type fakeMediaFileRepository struct {
	mu    sync.Mutex
	files map[string]*domain.MediaFile
}

func newFakeMediaFileRepository() *fakeMediaFileRepository {
	return &fakeMediaFileRepository{files: make(map[string]*domain.MediaFile)}
}

func (f *fakeMediaFileRepository) Create(_ context.Context, m *domain.MediaFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.files[m.ID]; exists {
		return ports.ErrConflict
	}
	cp := *m
	f.files[m.ID] = &cp
	return nil
}

func (f *fakeMediaFileRepository) Get(_ context.Context, id string) (*domain.MediaFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.files[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	cp := *m
	return &cp, nil
}

func (f *fakeMediaFileRepository) Update(_ context.Context, m *domain.MediaFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[m.ID]; !ok {
		return ports.ErrNotFound
	}
	cp := *m
	f.files[m.ID] = &cp
	return nil
}

func (f *fakeMediaFileRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.files, id)
	return nil
}

// List is unused by ScanExecutor's tests — present solely to satisfy
// ports.MediaFileRepository.
func (f *fakeMediaFileRepository) List(_ context.Context, itemID string, _ int, _ string) ([]*domain.MediaFile, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.MediaFile
	for _, m := range f.files {
		if itemID == "" || m.ItemID == itemID {
			cp := *m
			out = append(out, &cp)
		}
	}
	return out, "", nil
}

func (f *fakeMediaFileRepository) GetByHash(_ context.Context, oshash, sha1sum, md5sum, sha512sum string) (*domain.MediaFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, want := range []struct{ field, value string }{
		{"oshash", oshash}, {"sha1", sha1sum}, {"md5", md5sum}, {"sha512", sha512sum},
	} {
		if want.value == "" {
			continue
		}
		for _, m := range f.files {
			var got string
			switch want.field {
			case "oshash":
				got = m.OSHash
			case "sha1":
				got = m.SHA1
			case "md5":
				got = m.MD5
			case "sha512":
				got = m.SHA512
			}
			if got == want.value {
				cp := *m
				return &cp, nil
			}
		}
	}
	return nil, ports.ErrNotFound
}

func newEngine(t *testing.T, repo ports.UnmatchedFileRepository, mediaFileRepo ports.MediaFileRepository) *pkgjobqueue.Engine {
	t.Helper()
	engine := pkgjobqueue.NewEngine(memory.New())
	engine.Register("scan", pipeline.NewScanExecutor(repo, mediaFileRepo))
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
// Content is salted by name so distinct fixtures never collide by hash —
// which matters now that ScanExecutor actually performs hash lookups.
func writeHashableFixture(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, 70000)
	for i := range data {
		data[i] = byte(i) + name[0]
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
	mediaFileRepo := newFakeMediaFileRepository()
	engine := newEngine(t, repo, mediaFileRepo)

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
		if len(task.Steps) != 3 || task.Steps[0].Name != "hash" || task.Steps[1].Name != "check_known" || task.Steps[2].Name != "queue" {
			t.Fatalf("task %q has steps %+v, want [hash check_known queue]", task.Label, task.Steps)
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

		checkStep := findStep(task, "check_known")
		if checkStep.Detail["outcome"] != "new" {
			t.Errorf("task %q check_known outcome = %q, want %q", task.Label, checkStep.Detail["outcome"], "new")
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
	mediaFileRepo := newFakeMediaFileRepository()
	engine := newEngine(t, repo, mediaFileRepo)

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
	mediaFileRepo := newFakeMediaFileRepository()
	engine := newEngine(t, repo, mediaFileRepo)

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
	mediaFileRepo := newFakeMediaFileRepository()
	engine := newEngine(t, repo, mediaFileRepo)

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

	failingRepo := &alwaysFailUnmatchedFileRepository{err: errors.New("boom")}
	mediaFileRepo := newFakeMediaFileRepository()
	failEngine := pkgjobqueue.NewEngine(memory.New())
	failEngine.Register("scan", pipeline.NewScanExecutor(failingRepo, mediaFileRepo))

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

// TestScanExecutor_Execute_MatchesExistingMediaFile covers ADR-0024's
// "already known" short-circuit case 1: a file that matches an existing
// MediaFile by hash (already linked to a real Item, moved on disk). The
// check_known step must resolve matched_media_file, update the
// MediaFile's Path, and the queue step must never run.
func TestScanExecutor_Execute_MatchesExistingMediaFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := writeHashableFixture(t, dir, "old.flac")
	sha1sum, err := filehash.SHA1(oldPath)
	if err != nil {
		t.Fatalf("filehash.SHA1: %v", err)
	}

	mediaFileRepo := newFakeMediaFileRepository()
	mf := &domain.MediaFile{ID: "mf-1", ItemID: "item-1", Path: oldPath, SHA1: sha1sum}
	if err := mediaFileRepo.Create(context.Background(), mf); err != nil {
		t.Fatalf("mediaFileRepo.Create: %v", err)
	}

	// Simulate the file moving: same content (and thus hash) at a new path.
	newPath := filepath.Join(dir, "new.flac")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("os.Rename: %v", err)
	}

	repo := newFakeUnmatchedFileRepository()
	engine := newEngine(t, repo, mediaFileRepo)

	id, err := engine.Trigger(context.Background(), "scan", []string{newPath}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	task := job.Tasks[0]
	if len(task.Steps) != 2 || task.Steps[0].Name != "hash" || task.Steps[1].Name != "check_known" {
		t.Fatalf("task has steps %+v, want exactly [hash check_known] (queue must not run)", task.Steps)
	}

	checkStep := findStep(task, "check_known")
	if checkStep.Detail["outcome"] != "matched_media_file" {
		t.Fatalf("check_known outcome = %q, want %q", checkStep.Detail["outcome"], "matched_media_file")
	}
	if checkStep.Detail["media_file.id"] != "mf-1" {
		t.Fatalf("check_known media_file.id = %q, want %q", checkStep.Detail["media_file.id"], "mf-1")
	}

	got, err := mediaFileRepo.Get(context.Background(), "mf-1")
	if err != nil {
		t.Fatalf("mediaFileRepo.Get: %v", err)
	}
	if got.Path != newPath {
		t.Fatalf("MediaFile.Path = %q, want %q", got.Path, newPath)
	}
	if len(repo.files) != 0 {
		t.Fatalf("repo has %d UnmatchedFiles, want 0 (queue step must never have run)", len(repo.files))
	}
}

// TestScanExecutor_Execute_MatchesExistingUnmatchedFile covers ADR-0024's
// "already known" short-circuit case 2: a file that matches an existing
// UnmatchedFile by hash (still queued for review, moved on disk before
// anyone resolved it).
func TestScanExecutor_Execute_MatchesExistingUnmatchedFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := writeHashableFixture(t, dir, "old.flac")
	sha1sum, err := filehash.SHA1(oldPath)
	if err != nil {
		t.Fatalf("filehash.SHA1: %v", err)
	}

	repo := newFakeUnmatchedFileRepository()
	uf := &domain.UnmatchedFile{
		ID: "uf-1", Path: oldPath, GroupKey: oldPath, SHA1: sha1sum,
		DiscoveredAt: time.Now(), Status: domain.UnmatchedFileStatusPending,
	}
	if err := repo.Create(context.Background(), uf); err != nil {
		t.Fatalf("repo.Create: %v", err)
	}

	newPath := filepath.Join(dir, "new.flac")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("os.Rename: %v", err)
	}

	mediaFileRepo := newFakeMediaFileRepository()
	engine := newEngine(t, repo, mediaFileRepo)

	id, err := engine.Trigger(context.Background(), "scan", []string{newPath}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	task := job.Tasks[0]
	if len(task.Steps) != 2 || task.Steps[0].Name != "hash" || task.Steps[1].Name != "check_known" {
		t.Fatalf("task has steps %+v, want exactly [hash check_known] (queue must not run)", task.Steps)
	}

	checkStep := findStep(task, "check_known")
	if checkStep.Detail["outcome"] != "matched_unmatched_file" {
		t.Fatalf("check_known outcome = %q, want %q", checkStep.Detail["outcome"], "matched_unmatched_file")
	}
	if checkStep.Detail["unmatched_file.id"] != "uf-1" {
		t.Fatalf("check_known unmatched_file.id = %q, want %q", checkStep.Detail["unmatched_file.id"], "uf-1")
	}

	got, err := repo.Get(context.Background(), "uf-1")
	if err != nil {
		t.Fatalf("repo.Get: %v", err)
	}
	if got.Path != newPath {
		t.Fatalf("UnmatchedFile.Path = %q, want %q", got.Path, newPath)
	}
	if len(repo.files) != 1 {
		t.Fatalf("repo has %d UnmatchedFiles, want 1 (no new record created)", len(repo.files))
	}
}

// TestScanExecutor_Execute_CheckKnownRepositoryErrorFailsTask covers a
// MediaFileRepository error surfacing during the check_known step.
func TestScanExecutor_Execute_CheckKnownRepositoryErrorFailsTask(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	failingMediaFileRepo := &alwaysFailMediaFileRepository{err: errors.New("boom")}
	engine := pkgjobqueue.NewEngine(memory.New())
	engine.Register("scan", pipeline.NewScanExecutor(repo, failingMediaFileRepo))

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusFailed)
	}
	checkStep := findStep(job.Tasks[0], "check_known")
	if checkStep == nil || checkStep.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("check_known step = %+v, want a failed step", checkStep)
	}
	if findStep(job.Tasks[0], "queue") != nil {
		t.Fatal("queue step ran, want it skipped after check_known failed")
	}
}

type alwaysFailUnmatchedFileRepository struct {
	err error
}

func (a *alwaysFailUnmatchedFileRepository) Create(context.Context, *domain.UnmatchedFile) error {
	return a.err
}

func (a *alwaysFailUnmatchedFileRepository) Get(context.Context, string) (*domain.UnmatchedFile, error) {
	return nil, ports.ErrNotFound
}

func (a *alwaysFailUnmatchedFileRepository) Update(context.Context, *domain.UnmatchedFile) error {
	return a.err
}

func (a *alwaysFailUnmatchedFileRepository) Delete(context.Context, string) error {
	return a.err
}

func (a *alwaysFailUnmatchedFileRepository) List(context.Context, domain.UnmatchedFileStatus, int, string) ([]*domain.UnmatchedFile, string, error) {
	return nil, "", a.err
}

func (a *alwaysFailUnmatchedFileRepository) GetByHash(context.Context, string, string, string, string) (*domain.UnmatchedFile, error) {
	return nil, ports.ErrNotFound
}

func (a *alwaysFailUnmatchedFileRepository) UpdateBatch(context.Context, []*domain.UnmatchedFile) error {
	return a.err
}

func (a *alwaysFailUnmatchedFileRepository) ListByGroupKey(context.Context, string) ([]*domain.UnmatchedFile, error) {
	return nil, a.err
}

type alwaysFailMediaFileRepository struct {
	err error
}

func (a *alwaysFailMediaFileRepository) Create(context.Context, *domain.MediaFile) error {
	return a.err
}

func (a *alwaysFailMediaFileRepository) Get(context.Context, string) (*domain.MediaFile, error) {
	return nil, ports.ErrNotFound
}

func (a *alwaysFailMediaFileRepository) Update(context.Context, *domain.MediaFile) error {
	return a.err
}

func (a *alwaysFailMediaFileRepository) Delete(context.Context, string) error {
	return a.err
}

func (a *alwaysFailMediaFileRepository) List(context.Context, string, int, string) ([]*domain.MediaFile, string, error) {
	return nil, "", a.err
}

func (a *alwaysFailMediaFileRepository) GetByHash(context.Context, string, string, string, string) (*domain.MediaFile, error) {
	return nil, a.err
}
