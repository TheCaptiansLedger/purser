package pipeline_test

import (
	"context"
	"encoding/json"
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

// DeleteBatch removes every id atomically — used by ScanExecutor's
// decide/persist pass (Phase 6) once a group's winning candidate has been
// persisted.
func (f *fakeUnmatchedFileRepository) DeleteBatch(_ context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		if _, ok := f.files[id]; !ok {
			return ports.ErrNotFound
		}
	}
	for _, id := range ids {
		delete(f.files, id)
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

// fakeGroupingResolver is a minimal ports.GroupingResolver double.
// Returning a nil/empty groupResults map exercises ScanExecutor's own
// path-not-found fallback (GroupKey = path, DiscNumber = 0), matching
// today's pre-grouping identity behavior.
type fakeGroupingResolver struct {
	results map[string]ports.GroupingResult
	err     error
}

func (f *fakeGroupingResolver) GroupKeys(_ context.Context, _ domain.ContentType, paths []string) (map[string]ports.GroupingResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.results != nil {
		return f.results, nil
	}
	result := make(map[string]ports.GroupingResult, len(paths))
	for _, p := range paths {
		result[p] = ports.GroupingResult{GroupKey: p, DiscNumber: 0}
	}
	return result, nil
}

// fakeFileFingerprinterResolver is a minimal ports.FileFingerprinterResolver
// double. A zero-value instance behaves like service.NoopFingerprinter
// (empty Fingerprint from both methods, no error) so tests unrelated to
// fingerprinting can ignore it entirely. byPath lets a test return a
// distinct per-file Fingerprint; consensusCalls records every Consensus
// call's input so a test can assert exactly which Fingerprints were
// bucketed into a group.
type fakeFileFingerprinterResolver struct {
	mu             sync.Mutex
	byPath         map[string]domain.Fingerprint
	fingerprintErr error
	consensus      domain.Fingerprint
	consensusErr   error
	consensusCalls [][]domain.Fingerprint
}

func (f *fakeFileFingerprinterResolver) Fingerprint(_ context.Context, _ domain.ContentType, path string, _ int) (domain.Fingerprint, error) {
	if f.fingerprintErr != nil {
		return domain.Fingerprint{}, f.fingerprintErr
	}
	if f.byPath != nil {
		if fp, ok := f.byPath[path]; ok {
			return fp, nil
		}
	}
	return domain.Fingerprint{}, nil
}

func (f *fakeFileFingerprinterResolver) Consensus(_ context.Context, _ domain.ContentType, fingerprints []domain.Fingerprint) (domain.Fingerprint, error) {
	f.mu.Lock()
	f.consensusCalls = append(f.consensusCalls, fingerprints)
	f.mu.Unlock()
	if f.consensusErr != nil {
		return domain.Fingerprint{}, f.consensusErr
	}
	return f.consensus, nil
}

// fakeIdentifierResolver is a minimal ports.IdentifierResolver double —
// existing ScanExecutor tests don't exercise decide/persist, so this
// defaults to returning no candidates unless configured. errForGroupKey
// (optional) fails only the named group, so a test can prove one group's
// failure doesn't stop a sibling group in the same Job from being
// attempted — err (unqualified) fails every group, for tests that don't
// care about per-group isolation.
type fakeIdentifierResolver struct {
	mu             sync.Mutex
	candidates     []domain.MatchCandidate
	err            error
	errForGroupKey map[string]error
	calls          int
	calledGroups   []string
}

func (f *fakeIdentifierResolver) Identify(_ context.Context, _ domain.ContentType, _ domain.Fingerprint, _ []string, groupKey, _ string) ([]domain.MatchCandidate, error) {
	f.mu.Lock()
	f.calls++
	f.calledGroups = append(f.calledGroups, groupKey)
	f.mu.Unlock()
	if f.errForGroupKey != nil {
		if err, ok := f.errForGroupKey[groupKey]; ok {
			return nil, err
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.candidates, nil
}

// fakeConfidenceScoreResolver is a minimal ports.ConfidenceScoreResolver
// double — passes candidates through unscored unless scored is set.
type fakeConfidenceScoreResolver struct {
	scored []domain.MatchCandidate
	err    error
	calls  int
}

func (f *fakeConfidenceScoreResolver) ConfidenceScore(_ context.Context, _ domain.ContentType, _ domain.Fingerprint, candidates []domain.MatchCandidate) ([]domain.MatchCandidate, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.scored != nil {
		return f.scored, nil
	}
	return candidates, nil
}

// fakeDecisionResolver is a minimal pipeline.decisionResolver double —
// satisfied structurally, the unexported interface type is never named
// here. Defaults to persisted=false unless configured.
type fakeDecisionResolver struct {
	persisted bool
	err       error
	calls     int
	gotFiles  []*domain.UnmatchedFile
}

func (f *fakeDecisionResolver) Decide(_ context.Context, _ domain.ContentType, _ *domain.Fingerprint, _ []domain.MatchCandidate, files []*domain.UnmatchedFile) (bool, error) {
	f.calls++
	f.gotFiles = files
	if f.err != nil {
		return false, f.err
	}
	return f.persisted, nil
}

func newEngine(t *testing.T, repo ports.UnmatchedFileRepository, mediaFileRepo ports.MediaFileRepository) *pkgjobqueue.Engine {
	t.Helper()
	return newEngineWithGrouping(t, repo, mediaFileRepo, &fakeGroupingResolver{})
}

func newEngineWithGrouping(t *testing.T, repo ports.UnmatchedFileRepository, mediaFileRepo ports.MediaFileRepository, grouping ports.GroupingResolver) *pkgjobqueue.Engine {
	t.Helper()
	return newEngineWithFingerprinter(t, repo, mediaFileRepo, grouping, &fakeFileFingerprinterResolver{})
}

func newEngineWithFingerprinter(t *testing.T, repo ports.UnmatchedFileRepository, mediaFileRepo ports.MediaFileRepository, grouping ports.GroupingResolver, fingerprinter ports.FileFingerprinterResolver) *pkgjobqueue.Engine {
	t.Helper()
	return newEngineWithDecision(t, repo, mediaFileRepo, grouping, fingerprinter, &fakeIdentifierResolver{}, &fakeConfidenceScoreResolver{}, &fakeDecisionResolver{})
}

func newEngineWithDecision(
	t *testing.T,
	repo ports.UnmatchedFileRepository,
	mediaFileRepo ports.MediaFileRepository,
	grouping ports.GroupingResolver,
	fingerprinter ports.FileFingerprinterResolver,
	identifier ports.IdentifierResolver,
	confidenceScore ports.ConfidenceScoreResolver,
	decision *fakeDecisionResolver,
) *pkgjobqueue.Engine {
	t.Helper()
	engine := pkgjobqueue.NewEngine(memory.New())
	engine.Register("scan", pipeline.NewScanExecutor(repo, mediaFileRepo, grouping, fingerprinter, identifier, confidenceScore, decision))
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
		"content_type":  "music",
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
		if len(task.Steps) != 4 || task.Steps[0].Name != "hash" || task.Steps[1].Name != "fingerprint" || task.Steps[2].Name != "check_known" || task.Steps[3].Name != "queue" {
			t.Fatalf("task %q has steps %+v, want [hash fingerprint check_known queue]", task.Label, task.Steps)
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
		// No grouping was registered for this job's content type, so the
		// default fakeGroupingResolver (mirroring IdentityGrouping) means
		// GroupKey falls back to the file's own path.
		if got.GroupKey != task.Label {
			t.Errorf("stored UnmatchedFile.GroupKey = %q, want %q (identity fallback)", got.GroupKey, task.Label)
		}
		if got.DiscNumber != 0 {
			t.Errorf("stored UnmatchedFile.DiscNumber = %d, want 0 (identity fallback)", got.DiscNumber)
		}
	}
}

// TestScanExecutor_Execute_PropagatesGroupingResult covers the M3a wiring
// itself: a registered Grouping's result (a shared GroupKey across two
// files, a non-zero DiscNumber) must land on both created UnmatchedFile
// rows, replacing the old hardcoded GroupKey: path.
func TestScanExecutor_Execute_PropagatesGroupingResult(t *testing.T) {
	dir := t.TempDir()
	file1 := writeHashableFixture(t, dir, "one.flac")
	file2 := writeHashableFixture(t, dir, "two.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	grouping := &fakeGroupingResolver{results: map[string]ports.GroupingResult{
		file1: {GroupKey: "shared-album", DiscNumber: 2},
		file2: {GroupKey: "shared-album", DiscNumber: 2},
	}}
	engine := newEngineWithGrouping(t, repo, mediaFileRepo, grouping)

	id, err := engine.Trigger(context.Background(), "scan", []string{file1, file2}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	for _, task := range job.Tasks {
		ufID := findStep(task, "queue").Detail["unmatched_file.id"]
		got, err := repo.Get(context.Background(), ufID)
		if err != nil {
			t.Fatalf("repo.Get(%q) returned error: %v", ufID, err)
		}
		if got.GroupKey != "shared-album" {
			t.Errorf("task %q stored UnmatchedFile.GroupKey = %q, want %q", task.Label, got.GroupKey, "shared-album")
		}
		if got.DiscNumber != 2 {
			t.Errorf("task %q stored UnmatchedFile.DiscNumber = %d, want 2", task.Label, got.DiscNumber)
		}
	}
}

// TestScanExecutor_Execute_GroupingErrorFailsJob covers a GroupingResolver
// error: since grouping runs once for the whole Job before any task
// starts, a failure there must fail the Job outright rather than being
// silently ignored per task.
func TestScanExecutor_Execute_GroupingErrorFailsJob(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	grouping := &fakeGroupingResolver{err: errors.New("boom")}
	engine := newEngineWithGrouping(t, repo, mediaFileRepo, grouping)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusFailed)
	}
	if len(repo.files) != 0 {
		t.Fatalf("repo has %d files, want 0 (no task should have run)", len(repo.files))
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
		"content_type":  "music",
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
	failEngine.Register("scan", pipeline.NewScanExecutor(failingRepo, mediaFileRepo, &fakeGroupingResolver{}, &fakeFileFingerprinterResolver{}, &fakeIdentifierResolver{}, &fakeConfidenceScoreResolver{}, &fakeDecisionResolver{}))

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
	if len(task.Steps) != 3 || task.Steps[0].Name != "hash" || task.Steps[1].Name != "fingerprint" || task.Steps[2].Name != "check_known" {
		t.Fatalf("task has steps %+v, want exactly [hash fingerprint check_known] (queue must not run)", task.Steps)
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
	if len(task.Steps) != 3 || task.Steps[0].Name != "hash" || task.Steps[1].Name != "fingerprint" || task.Steps[2].Name != "check_known" {
		t.Fatalf("task has steps %+v, want exactly [hash fingerprint check_known] (queue must not run)", task.Steps)
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
	engine.Register("scan", pipeline.NewScanExecutor(repo, failingMediaFileRepo, &fakeGroupingResolver{}, &fakeFileFingerprinterResolver{}, &fakeIdentifierResolver{}, &fakeConfidenceScoreResolver{}, &fakeDecisionResolver{}))

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

func (a *alwaysFailUnmatchedFileRepository) DeleteBatch(context.Context, []string) error {
	return a.err
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

// TestScanExecutor_Execute_FingerprintStepRecordsTagCount covers the
// "fingerprint" Step's happy path: the FileFingerprinterResolver's result
// is recorded on the Step (tag_count) and the Task still proceeds normally
// through check_known/queue.
func TestScanExecutor_Execute_FingerprintStepRecordsTagCount(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	fingerprinter := &fakeFileFingerprinterResolver{
		byPath: map[string]domain.Fingerprint{
			file: {Tags: map[string]string{"ALBUM": "Test Album", "TITLE": "Test Title"}},
		},
	}
	engine := newEngineWithFingerprinter(t, repo, mediaFileRepo, &fakeGroupingResolver{}, fingerprinter)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	fpStep := findStep(job.Tasks[0], "fingerprint")
	if fpStep == nil || fpStep.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("fingerprint step = %+v, want a succeeded step", fpStep)
	}
	if fpStep.Detail["tag_count"] != "2" {
		t.Errorf("fingerprint step tag_count = %q, want %q", fpStep.Detail["tag_count"], "2")
	}
}

// TestScanExecutor_Execute_FingerprintFailureIsNonFatal covers
// docs/technical/pipeline-music-fingerprinter.md's implicit requirement
// that one file's unreadable/corrupt fingerprint never blocks it from
// still being queued for review: the fingerprint Step is recorded as
// failed, but check_known/queue still run and the Job still succeeds.
func TestScanExecutor_Execute_FingerprintFailureIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	fingerprinter := &fakeFileFingerprinterResolver{fingerprintErr: errors.New("ffprobe: boom")}
	engine := newEngineWithFingerprinter(t, repo, mediaFileRepo, &fakeGroupingResolver{}, fingerprinter)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q (fingerprint failure must be non-fatal)", job.Status, pkgjobqueue.StatusSucceeded)
	}

	task := job.Tasks[0]
	fpStep := findStep(task, "fingerprint")
	if fpStep == nil || fpStep.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("fingerprint step = %+v, want a failed step", fpStep)
	}
	if findStep(task, "queue") == nil {
		t.Fatal("queue step did not run, want it to run despite the fingerprint failure")
	}
	if len(repo.files) != 1 {
		t.Fatalf("repo has %d files, want 1 (still queued despite fingerprint failure)", len(repo.files))
	}
}

// TestScanExecutor_Execute_PersistsConsensusOntoGroup covers the pass-2
// group consensus write: once every Task in the Job has run, the buffered
// per-file Fingerprints for a shared GroupKey are reduced via Consensus and
// written onto every UnmatchedFile row in that group with one UpdateBatch
// call.
// TestScanExecutor_Execute_QueuesRealTrackPosition covers a gap found
// during #518: the per-file disc_number/track_number a FileFingerprinter
// already computes (DISCNUMBER-tag override, TRACKNUMBER carried raw) was
// never read back onto the queued UnmatchedFile row — DiscNumber stayed at
// grouping's folder-only guess and TrackNumber was always empty, which
// would have made the Music Persister's tracklist matching
// (docs/technical/pipeline-music-persist.md's step 6) never match a single
// real scanned file.
func TestScanExecutor_Execute_QueuesRealTrackPosition(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	grouping := &fakeGroupingResolver{results: map[string]ports.GroupingResult{
		file: {GroupKey: "shared-album", DiscNumber: 1}, // folder-derived guess
	}}
	fingerprinter := &fakeFileFingerprinterResolver{
		byPath: map[string]domain.Fingerprint{
			file: {
				Tags:     map[string]string{"ALBUM": "Test Album"},
				Metadata: map[string]any{"disc_number": 2, "track_number": "B3"}, // DISCNUMBER tag overrides the folder guess; vinyl side-lettering preserved
			},
		},
	}
	engine := newEngineWithFingerprinter(t, repo, mediaFileRepo, grouping, fingerprinter)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	ufID := findStep(job.Tasks[0], "queue").Detail["unmatched_file.id"]
	got, err := repo.Get(context.Background(), ufID)
	if err != nil {
		t.Fatalf("repo.Get(%q) returned error: %v", ufID, err)
	}
	if got.DiscNumber != 2 {
		t.Errorf("UnmatchedFile.DiscNumber = %d, want %d (the DISCNUMBER-tag override, not the folder-derived guess of 1)", got.DiscNumber, 2)
	}
	if got.TrackNumber != "B3" {
		t.Errorf("UnmatchedFile.TrackNumber = %q, want %q (carried raw from the TRACKNUMBER tag)", got.TrackNumber, "B3")
	}
}

func TestScanExecutor_Execute_PersistsConsensusOntoGroup(t *testing.T) {
	dir := t.TempDir()
	file1 := writeHashableFixture(t, dir, "one.flac")
	file2 := writeHashableFixture(t, dir, "two.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	grouping := &fakeGroupingResolver{results: map[string]ports.GroupingResult{
		file1: {GroupKey: "shared-album", DiscNumber: 1},
		file2: {GroupKey: "shared-album", DiscNumber: 1},
	}}
	fingerprinter := &fakeFileFingerprinterResolver{
		byPath: map[string]domain.Fingerprint{
			file1: {Tags: map[string]string{"ALBUM": "Test Album"}},
			file2: {Tags: map[string]string{"ALBUM": "Test Album"}},
		},
		consensus: domain.Fingerprint{
			Tags:     map[string]string{"ALBUM": "Consensus Album"},
			Metadata: map[string]any{"track_count": 2},
		},
	}
	engine := newEngineWithFingerprinter(t, repo, mediaFileRepo, grouping, fingerprinter)

	id, err := engine.Trigger(context.Background(), "scan", []string{file1, file2}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	if len(fingerprinter.consensusCalls) != 1 || len(fingerprinter.consensusCalls[0]) != 2 {
		t.Fatalf("Consensus called with %+v, want exactly one call with 2 fingerprints", fingerprinter.consensusCalls)
	}

	for _, task := range job.Tasks {
		ufID := findStep(task, "queue").Detail["unmatched_file.id"]
		got, err := repo.Get(context.Background(), ufID)
		if err != nil {
			t.Fatalf("repo.Get(%q) returned error: %v", ufID, err)
		}
		if got.Fingerprint == nil {
			t.Fatalf("task %q stored UnmatchedFile.Fingerprint is nil, want the group consensus", task.Label)
		}
		if got.Fingerprint.Tags["ALBUM"] != "Consensus Album" {
			t.Errorf("task %q Fingerprint.Tags[ALBUM] = %q, want %q", task.Label, got.Fingerprint.Tags["ALBUM"], "Consensus Album")
		}
	}
}

// TestScanExecutor_Execute_SkipsConsensusWriteWhenEmpty covers the "no
// registered fingerprinter" case (service.NoopFingerprinter, mirrored here
// by the fake's zero-value behavior): Consensus returns an empty
// Fingerprint, and ScanExecutor must not write it onto UnmatchedFile rows —
// content types this feature doesn't touch yet get no writes at all.
func TestScanExecutor_Execute_SkipsConsensusWriteWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	engine := newEngine(t, repo, mediaFileRepo)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	ufID := findStep(job.Tasks[0], "queue").Detail["unmatched_file.id"]
	got, err := repo.Get(context.Background(), ufID)
	if err != nil {
		t.Fatalf("repo.Get(%q) returned error: %v", ufID, err)
	}
	if got.Fingerprint != nil {
		t.Fatalf("stored UnmatchedFile.Fingerprint = %+v, want nil (empty consensus must not be written)", got.Fingerprint)
	}
}

// TestScanExecutor_Execute_SkipsDecideWhenConsensusEmpty extends the above:
// a content type with no real fingerprinter registered must not attempt
// identify/score/decide at all, the same "no writes for untouched content
// types" rule already applies to the consensus write itself.
func TestScanExecutor_Execute_SkipsDecideWhenConsensusEmpty(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	identifier := &fakeIdentifierResolver{}
	confidenceScore := &fakeConfidenceScoreResolver{}
	decision := &fakeDecisionResolver{}
	engine := newEngineWithDecision(t, repo, mediaFileRepo, &fakeGroupingResolver{}, &fakeFileFingerprinterResolver{}, identifier, confidenceScore, decision)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	if identifier.calls != 0 || confidenceScore.calls != 0 || decision.calls != 0 {
		t.Fatalf("identify/score/decide called %d/%d/%d times, want 0/0/0 for an empty consensus", identifier.calls, confidenceScore.calls, decision.calls)
	}
}

// TestScanExecutor_Execute_DecideAndPersist_AutoImportDeletesGroup covers
// the automatic path end to end: a group whose consensus clears the
// (fake) decision's threshold is persisted and its UnmatchedFile rows
// deleted out of the review queue — the same "leaves the queue by
// deletion" behavior Resolve/AcceptCandidate already have, applied
// automatically. The "decide" Step is recorded on the task even though it
// already reached a terminal status earlier in the same Job.
func TestScanExecutor_Execute_DecideAndPersist_AutoImportDeletesGroup(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	fingerprinter := &fakeFileFingerprinterResolver{
		byPath:    map[string]domain.Fingerprint{file: {Tags: map[string]string{"ALBUM": "Test Album"}}},
		consensus: domain.Fingerprint{Tags: map[string]string{"ALBUM": "Consensus Album"}},
	}
	candidate := domain.MatchCandidate{ExternalRef: "release-mbid-1", Score: 0.95}
	identifier := &fakeIdentifierResolver{candidates: []domain.MatchCandidate{candidate}}
	confidenceScore := &fakeConfidenceScoreResolver{}
	decision := &fakeDecisionResolver{persisted: true}
	engine := newEngineWithDecision(t, repo, mediaFileRepo, &fakeGroupingResolver{}, fingerprinter, identifier, confidenceScore, decision)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	if identifier.calls != 1 || confidenceScore.calls != 1 || decision.calls != 1 {
		t.Fatalf("identify/score/decide called %d/%d/%d times, want 1/1/1", identifier.calls, confidenceScore.calls, decision.calls)
	}
	if len(decision.gotFiles) != 1 {
		t.Fatalf("Decide called with %d files, want 1", len(decision.gotFiles))
	}

	ufID := findStep(job.Tasks[0], "queue").Detail["unmatched_file.id"]
	if _, err := repo.Get(context.Background(), ufID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("repo.Get after a persisted decide returned %v, want ErrNotFound (the group must be deleted)", err)
	}

	decideStep := findStep(job.Tasks[0], "decide")
	if decideStep == nil || decideStep.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("decide step = %+v, want a succeeded step", decideStep)
	}
	if decideStep.Detail["persisted"] != "true" {
		t.Errorf("decide step Detail[persisted] = %q, want %q", decideStep.Detail["persisted"], "true")
	}
	if decideStep.Detail["candidate_count"] != "1" {
		t.Errorf("decide step Detail[candidate_count] = %q, want %q", decideStep.Detail["candidate_count"], "1")
	}

	// The full scored candidate list — ExternalRef, Score, Tier, Signals —
	// lives on the Job itself (docs/adr/0023-job-queue.md's own framing of
	// Step.Detail: "a matched MBID, a computed confidence score"), so
	// answering "why did/didn't this group auto-import" never needs a
	// temporary debug log bolted onto the pipeline.
	var gotCandidates []domain.MatchCandidate
	if err := json.Unmarshal([]byte(decideStep.Detail["candidates"]), &gotCandidates); err != nil {
		t.Fatalf("decoding decide step Detail[candidates] returned error: %v (raw: %q)", err, decideStep.Detail["candidates"])
	}
	if len(gotCandidates) != 1 || gotCandidates[0].ExternalRef != "release-mbid-1" || gotCandidates[0].Score != 0.95 {
		t.Errorf("decide step Detail[candidates] decoded to %+v, want one candidate matching %+v", gotCandidates, candidate)
	}
}

// TestScanExecutor_Execute_DecideAndPersist_BelowThresholdLeavesGroupPending
// covers the inverse: a group whose decision doesn't clear the threshold
// stays in the review queue, untouched.
func TestScanExecutor_Execute_DecideAndPersist_BelowThresholdLeavesGroupPending(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	fingerprinter := &fakeFileFingerprinterResolver{
		byPath:    map[string]domain.Fingerprint{file: {Tags: map[string]string{"ALBUM": "Test Album"}}},
		consensus: domain.Fingerprint{Tags: map[string]string{"ALBUM": "Consensus Album"}},
	}
	candidate := domain.MatchCandidate{ExternalRef: "release-mbid-1", Score: 0.70, Tier: domain.MatchTierFuzzy}
	identifier := &fakeIdentifierResolver{candidates: []domain.MatchCandidate{candidate}}
	confidenceScore := &fakeConfidenceScoreResolver{}
	decision := &fakeDecisionResolver{persisted: false}
	engine := newEngineWithDecision(t, repo, mediaFileRepo, &fakeGroupingResolver{}, fingerprinter, identifier, confidenceScore, decision)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusSucceeded {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusSucceeded)
	}

	ufID := findStep(job.Tasks[0], "queue").Detail["unmatched_file.id"]
	got, err := repo.Get(context.Background(), ufID)
	if err != nil {
		t.Fatalf("repo.Get after a below-threshold decide returned error: %v, want the row still present", err)
	}
	if got.Status != domain.UnmatchedFileStatusPending {
		t.Fatalf("UnmatchedFile.Status = %q, want %q (untouched)", got.Status, domain.UnmatchedFileStatusPending)
	}
	// The real scored candidate must land on the row — a caller
	// (GetUnmatchedFile/ListUnmatchedFiles/ListGroupUnmatchedFiles) needs
	// to actually see the match and its confidence score to let a human
	// accept it, per AcceptCandidateRequest's own doc comment ("a human
	// picks any ranked candidate"). This used to come back empty every
	// time — the scored candidate was computed, then thrown away.
	if len(got.Candidates) != 1 || got.Candidates[0].ExternalRef != candidate.ExternalRef || got.Candidates[0].Score != candidate.Score {
		t.Fatalf("UnmatchedFile.Candidates = %+v, want [%+v]", got.Candidates, candidate)
	}

	decideStep := findStep(job.Tasks[0], "decide")
	if decideStep == nil || decideStep.Detail["persisted"] != "false" {
		t.Fatalf("decide step = %+v, want Detail[persisted]=false", decideStep)
	}
}

// TestScanExecutor_Execute_OneGroupsDecideFailureDoesNotBlockSiblingGroup
// covers decideAndPersistGroups' groupBusinessError isolation: one group's
// identify/score/decide failure (a MusicBrainz timeout, in production)
// must not stop a sibling group in the same Job from getting its own
// independent identify/score/decide attempt and, on success, being
// auto-imported — a regression test for purser#522's manual verification
// finding that a single group's failure was silently swallowing every
// other group's Job.
func TestScanExecutor_Execute_OneGroupsDecideFailureDoesNotBlockSiblingGroup(t *testing.T) {
	dir := t.TempDir()
	failFile := writeHashableFixture(t, dir, "fails.flac")
	okFile := writeHashableFixture(t, dir, "ok.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	fingerprinter := &fakeFileFingerprinterResolver{
		byPath: map[string]domain.Fingerprint{
			failFile: {Tags: map[string]string{"ALBUM": "Fails"}},
			okFile:   {Tags: map[string]string{"ALBUM": "OK"}},
		},
		consensus: domain.Fingerprint{Tags: map[string]string{"ALBUM": "Consensus"}},
	}
	candidate := domain.MatchCandidate{ExternalRef: "release-mbid-1", Score: 0.95}
	identifier := &fakeIdentifierResolver{
		candidates:     []domain.MatchCandidate{candidate},
		errForGroupKey: map[string]error{failFile: errors.New("musicbrainz: timeout awaiting response headers")},
	}
	confidenceScore := &fakeConfidenceScoreResolver{}
	decision := &fakeDecisionResolver{persisted: true}
	// Default fakeGroupingResolver puts every path in its own group
	// (GroupKey == path), so failFile and okFile are two independent
	// groups in the same Job.
	engine := newEngineWithDecision(t, repo, mediaFileRepo, &fakeGroupingResolver{}, fingerprinter, identifier, confidenceScore, decision)

	id, err := engine.Trigger(context.Background(), "scan", []string{failFile, okFile}, map[string]string{"content_type": "music"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	job := waitForTerminal(t, engine, id.ID)

	if identifier.calls != 2 {
		t.Fatalf("Identify called %d times, want 2 — okFile's group must still be attempted after failFile's group fails", identifier.calls)
	}
	// The Job as a whole still reports failure — one of its groups
	// genuinely needs attention — but that's now an accurate summary of
	// a partial outcome, not a report that nothing happened at all.
	if job.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("job.Status = %q, want %q (one group failed)", job.Status, pkgjobqueue.StatusFailed)
	}

	var failTask, okTask *pkgjobqueue.Task
	for _, task := range job.Tasks {
		switch task.Label {
		case failFile:
			failTask = task
		case okFile:
			okTask = task
		}
	}
	if failTask == nil || okTask == nil {
		t.Fatalf("expected tasks for both %q and %q, got %d tasks", failFile, okFile, len(job.Tasks))
	}

	failDecide := findStep(failTask, "decide")
	if failDecide == nil || failDecide.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("failFile's decide step = %+v, want a failed step", failDecide)
	}
	okDecide := findStep(okTask, "decide")
	if okDecide == nil || okDecide.Status != pkgjobqueue.StatusSucceeded || okDecide.Detail["persisted"] != "true" {
		t.Fatalf("okFile's decide step = %+v, want a succeeded step with persisted=true", okDecide)
	}

	// okFile's group was auto-imported (its row deleted); failFile's
	// group is still sitting in the review queue, untouched.
	okUfID := findStep(okTask, "queue").Detail["unmatched_file.id"]
	if _, err := repo.Get(context.Background(), okUfID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("repo.Get(okFile's row) after a persisted decide returned %v, want ErrNotFound", err)
	}
	failUfID := findStep(failTask, "queue").Detail["unmatched_file.id"]
	if _, err := repo.Get(context.Background(), failUfID); err != nil {
		t.Fatalf("repo.Get(failFile's row) after a failed decide returned error: %v, want the row still present", err)
	}
}

// TestScanExecutor_Execute_ConsensusErrorFailsJob covers a Consensus error
// during the post-loop pass: it must fail the Job, since it happens outside
// any single Task's lifecycle.
func TestScanExecutor_Execute_ConsensusErrorFailsJob(t *testing.T) {
	dir := t.TempDir()
	file := writeHashableFixture(t, dir, "one.flac")

	repo := newFakeUnmatchedFileRepository()
	mediaFileRepo := newFakeMediaFileRepository()
	fingerprinter := &fakeFileFingerprinterResolver{
		byPath:       map[string]domain.Fingerprint{file: {Tags: map[string]string{"ALBUM": "X"}}},
		consensusErr: errors.New("boom"),
	}
	engine := newEngineWithFingerprinter(t, repo, mediaFileRepo, &fakeGroupingResolver{}, fingerprinter)

	id, err := engine.Trigger(context.Background(), "scan", []string{file}, nil)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	job := waitForTerminal(t, engine, id.ID)
	if job.Status != pkgjobqueue.StatusFailed {
		t.Fatalf("job.Status = %q, want %q", job.Status, pkgjobqueue.StatusFailed)
	}
}
