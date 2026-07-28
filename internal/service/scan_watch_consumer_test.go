package service_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"purser/internal/ports"
	"purser/internal/service"
	"purser/pkg/fswatch"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeFileWatcher is a minimal in-memory ports.FileWatcher double — zero
// knowledge of the real pkg/fswatch.Watcher orchestration, per
// docs/adr/0001-hexagonal-architecture.md's "a port must be satisfiable by
// a fake with zero knowledge of any real adapter."
type fakeFileWatcher struct {
	events chan fswatch.Event
	errs   chan error
}

func newFakeFileWatcher() *fakeFileWatcher {
	return &fakeFileWatcher{
		events: make(chan fswatch.Event, 8),
		errs:   make(chan error, 8),
	}
}

func (f *fakeFileWatcher) Events() <-chan fswatch.Event { return f.events }
func (f *fakeFileWatcher) Errors() <-chan error         { return f.errs }

// watchFakePublisher is a ports.JobPublisher double recording every
// Trigger call's root (its taskLabels[0] convention doesn't apply here —
// ScanWatchConsumer calls ScanService.Trigger, which passes kind/labels
// derived from the walk, not the root itself, so this fake instead
// records the root ScanWatchConsumer asked ScanService to scan via a
// wrapping fakeFileWalker; see triggeredRoots below). Safe for concurrent
// use — ScanWatchConsumer.Run's goroutine calls Trigger while the test
// goroutine reads.
type watchFakePublisher struct {
	mu       sync.Mutex
	returnID string
	calls    int
}

func newWatchFakePublisher(returnID string) *watchFakePublisher {
	return &watchFakePublisher{returnID: returnID}
}

func (f *watchFakePublisher) Trigger(_ context.Context, _ string, _ []string, _ map[string]string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.returnID, nil
}

func (f *watchFakePublisher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// watchFakeWalker records every root ScanService.Trigger was asked to
// walk — this is how the test observes which unit path
// ScanWatchConsumer.handleEvent passed through, since ScanService.Trigger
// itself doesn't return the root it was given.
type watchFakeWalker struct {
	mu    sync.Mutex
	roots []string
}

func (f *watchFakeWalker) Walk(_ context.Context, root string) ([]ports.DiscoveredFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.roots = append(f.roots, root)
	return nil, nil
}

func (f *watchFakeWalker) seenRoots() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.roots))
	copy(out, f.roots)
	return out
}

func waitForCallCount(t *testing.T, pub *watchFakePublisher, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pub.callCount() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d Trigger call(s), got %d", want, pub.callCount())
}

// syncBuffer is a mutex-guarded bytes.Buffer — consumer.Run's goroutine
// writes log records to it concurrently with the test goroutine polling
// String(), which a plain bytes.Buffer doesn't allow race-free.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

var errTestWatch = errors.New("boom: watch source failed")

func TestScanWatchConsumer_Run_TriggersScanOnSettledEvent(t *testing.T) {
	watcher := newFakeFileWatcher()
	pub := newWatchFakePublisher("job-1")
	walker := &watchFakeWalker{}
	scanSvc := service.NewScanService(pub, walker, false, false, nil, &fakeSidecarClassifierResolver{})
	consumer := service.NewScanWatchConsumer(watcher, scanSvc, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go consumer.Run(ctx)

	watcher.events <- fswatch.Event{Root: "/media", Path: "/media/new-album", Kind: fswatch.Created}

	waitForCallCount(t, pub, 1)
	roots := walker.seenRoots()
	if len(roots) != 1 || roots[0] != "/media/new-album" {
		t.Fatalf("ScanService.Trigger walked roots %v, want [/media/new-album]", roots)
	}
}

func TestScanWatchConsumer_Run_SkipsRemovedEvent(t *testing.T) {
	watcher := newFakeFileWatcher()
	pub := newWatchFakePublisher("job-1")
	walker := &watchFakeWalker{}
	scanSvc := service.NewScanService(pub, walker, false, false, nil, &fakeSidecarClassifierResolver{})
	consumer := service.NewScanWatchConsumer(watcher, scanSvc, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go consumer.Run(ctx)

	watcher.events <- fswatch.Event{Root: "/media", Path: "/media/gone", Kind: fswatch.Removed}
	// Follow with a real event so there's a positive signal the loop kept
	// processing, instead of asserting a timing-dependent absence.
	watcher.events <- fswatch.Event{Root: "/media", Path: "/media/new-album", Kind: fswatch.Created}

	waitForCallCount(t, pub, 1)
	roots := walker.seenRoots()
	if len(roots) != 1 || roots[0] != "/media/new-album" {
		t.Fatalf("ScanService.Trigger walked roots %v, want exactly one call for /media/new-album (Removed must not trigger)", roots)
	}
}

func TestScanWatchConsumer_Run_LogsWatcherErrorAndContinues(t *testing.T) {
	watcher := newFakeFileWatcher()
	pub := newWatchFakePublisher("job-1")
	walker := &watchFakeWalker{}
	scanSvc := service.NewScanService(pub, walker, false, false, nil, &fakeSidecarClassifierResolver{})

	var logBuf syncBuffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	consumer := service.NewScanWatchConsumer(watcher, scanSvc, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go consumer.Run(ctx)

	watcher.errs <- errTestWatch
	// Confirm the loop survived the error by triggering a scan afterwards.
	watcher.events <- fswatch.Event{Root: "/media", Path: "/media/new-album", Kind: fswatch.Created}
	waitForCallCount(t, pub, 1)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !strings.Contains(logBuf.String(), "filesystem watch error") {
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(logBuf.String(), "filesystem watch error") {
		t.Fatalf("log output %q does not contain watcher error message", logBuf.String())
	}
	if !strings.Contains(logBuf.String(), errTestWatch.Error()) {
		t.Fatalf("log output %q does not contain the error value", logBuf.String())
	}
}

func TestScanWatchConsumer_Run_StopsOnContextCancel(t *testing.T) {
	watcher := newFakeFileWatcher()
	pub := newWatchFakePublisher("job-1")
	walker := &watchFakeWalker{}
	scanSvc := service.NewScanService(pub, walker, false, false, nil, &fakeSidecarClassifierResolver{})
	consumer := service.NewScanWatchConsumer(watcher, scanSvc, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		consumer.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
