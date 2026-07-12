package fswatch_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"purser/pkg/fswatch"
	"purser/pkg/fswatch/fswatchtest"
	"sync"
	"testing"
	"time"
)

// startTestWatcher wires a Watcher to a fswatchtest.Fake source and starts
// it against root. cfg.SettleWindow/MaxWait are expected to already be set
// to short, test-appropriate durations by the caller.
func startTestWatcher(t *testing.T, root string, cfg fswatch.Config) (*fswatch.Watcher, *fswatchtest.Fake) {
	t.Helper()

	src := fswatchtest.NewFake()
	w, err := fswatch.New(src, []string{root}, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = w.Close()
	})

	return w, src
}

func waitForUnitEvent(t *testing.T, ch <-chan fswatch.Event, timeout time.Duration) fswatch.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(timeout):
		t.Fatal("timed out waiting for an Event")
		return fswatch.Event{}
	}
}

func expectNoEvent(t *testing.T, ch <-chan fswatch.Event, within time.Duration) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("received unexpected event: %+v", ev)
	case <-time.After(within):
	}
}

// mkdirReal + writeReal create real filesystem entries so registerSubtree's
// os.Stat/filepath.WalkDir calls (which always hit the real filesystem
// regardless of which Source is injected) see what the test's synthetic
// RawEvents describe.
func mkdirReal(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}

func writeReal(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestWatcher_BurstWithinWindowCoalescesToOneEvent(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 2
	cfg.SettleWindow = 40 * time.Millisecond
	cfg.MaxWait = 2 * time.Second

	w, src := startTestWatcher(t, root, cfg)

	albumDir := filepath.Join(root, "Artist", "Album")
	mkdirReal(t, albumDir)
	src.Emit(fswatch.RawEvent{Path: filepath.Join(root, "Artist"), Op: fswatch.OpCreate})
	src.Emit(fswatch.RawEvent{Path: albumDir, Op: fswatch.OpCreate})

	tracks := []string{"01.flac", "02.flac", "03.flac"}
	for _, name := range tracks {
		p := filepath.Join(albumDir, name)
		writeReal(t, p, "data")
		src.Emit(fswatch.RawEvent{Path: p, Op: fswatch.OpCreate})
		time.Sleep(5 * time.Millisecond) // stay well within SettleWindow
	}

	ev := waitForUnitEvent(t, w.Events(), 2*time.Second)

	if ev.Path != albumDir {
		t.Fatalf("Path = %q, want %q", ev.Path, albumDir)
	}
	if ev.Kind != fswatch.Created {
		t.Fatalf("Kind = %v, want Created", ev.Kind)
	}
	if len(ev.Files) != len(tracks) {
		t.Fatalf("Files = %v, want %d entries", ev.Files, len(tracks))
	}
	seen := make(map[string]bool)
	for _, f := range ev.Files {
		if seen[f] {
			t.Fatalf("duplicate file in Files: %q", f)
		}
		seen[f] = true
	}

	expectNoEvent(t, w.Events(), 150*time.Millisecond)
}

func TestWatcher_MaxWaitFiresDespiteContinuousActivity(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 1
	cfg.SettleWindow = 60 * time.Millisecond
	cfg.MaxWait = 150 * time.Millisecond

	w, src := startTestWatcher(t, root, cfg)

	unitDir := filepath.Join(root, "Title")
	mkdirReal(t, unitDir)
	src.Emit(fswatch.RawEvent{Path: unitDir, Op: fswatch.OpCreate})

	start := time.Now()
	stop := time.After(400 * time.Millisecond)
	i := 0
loop:
	for {
		select {
		case <-stop:
			break loop
		default:
			p := filepath.Join(unitDir, fmt.Sprintf("part-%d", i))
			writeReal(t, p, "x")
			src.Emit(fswatch.RawEvent{Path: p, Op: fswatch.OpCreate})
			i++
			time.Sleep(30 * time.Millisecond) // < SettleWindow: never goes quiet
		}
	}

	ev := waitForUnitEvent(t, w.Events(), 2*time.Second)
	elapsed := time.Since(start)

	if ev.Path != unitDir {
		t.Fatalf("Path = %q, want %q", ev.Path, unitDir)
	}
	// Must fire at/after MaxWait, and must not be starved indefinitely by
	// continuous activity resetting SettleWindow forever.
	if elapsed < cfg.MaxWait {
		t.Fatalf("fired after %s, before MaxWait (%s)", elapsed, cfg.MaxWait)
	}
	if elapsed > cfg.MaxWait+300*time.Millisecond {
		t.Fatalf("fired after %s, too long past MaxWait (%s)", elapsed, cfg.MaxWait)
	}
}

func TestWatcher_MultiDiscSubfolderRollsUpToAlbumUnit(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 2
	cfg.SettleWindow = 40 * time.Millisecond
	cfg.MaxWait = 2 * time.Second

	w, src := startTestWatcher(t, root, cfg)

	albumDir := filepath.Join(root, "Artist", "Album")
	discDir := filepath.Join(albumDir, "Disc 1")
	mkdirReal(t, discDir)

	src.Emit(fswatch.RawEvent{Path: filepath.Join(root, "Artist"), Op: fswatch.OpCreate})
	src.Emit(fswatch.RawEvent{Path: albumDir, Op: fswatch.OpCreate})
	src.Emit(fswatch.RawEvent{Path: discDir, Op: fswatch.OpCreate})

	trackPath := filepath.Join(discDir, "01.flac")
	writeReal(t, trackPath, "data")
	src.Emit(fswatch.RawEvent{Path: trackPath, Op: fswatch.OpCreate})

	coverPath := filepath.Join(albumDir, "cover.jpg")
	writeReal(t, coverPath, "data")
	src.Emit(fswatch.RawEvent{Path: coverPath, Op: fswatch.OpCreate})

	ev := waitForUnitEvent(t, w.Events(), 2*time.Second)
	if ev.Path != albumDir {
		t.Fatalf("Path = %q, want %q (disc subfolder must roll up into the album unit)", ev.Path, albumDir)
	}

	found := map[string]bool{}
	for _, f := range ev.Files {
		found[f] = true
	}
	if !found[trackPath] || !found[coverPath] {
		t.Fatalf("Files = %v, want both %q and %q", ev.Files, trackPath, coverPath)
	}

	expectNoEvent(t, w.Events(), 150*time.Millisecond)
}

func TestWatcher_DeletionFiresImmediatelyAndCancelsPending(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 1
	cfg.SettleWindow = 300 * time.Millisecond // long enough that a stray second fire would be caught
	cfg.MaxWait = 2 * time.Second

	w, src := startTestWatcher(t, root, cfg)

	unitDir := filepath.Join(root, "Title")
	mkdirReal(t, unitDir)
	src.Emit(fswatch.RawEvent{Path: unitDir, Op: fswatch.OpCreate})

	trackPath := filepath.Join(unitDir, "file.mp4")
	writeReal(t, trackPath, "data")
	src.Emit(fswatch.RawEvent{Path: trackPath, Op: fswatch.OpCreate})

	// Remove before SettleWindow elapses — the pending Created must be
	// cancelled, not delivered, and Removed must fire without waiting.
	if err := os.RemoveAll(unitDir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	src.Emit(fswatch.RawEvent{Path: unitDir, Op: fswatch.OpRemove})

	ev := waitForUnitEvent(t, w.Events(), 200*time.Millisecond)
	if ev.Kind != fswatch.Removed {
		t.Fatalf("Kind = %v, want Removed", ev.Kind)
	}
	if ev.Path != unitDir {
		t.Fatalf("Path = %q, want %q", ev.Path, unitDir)
	}

	// The cancelled Created must never arrive — wait well past the
	// original SettleWindow to prove it, catching a duplicate/stale fire.
	expectNoEvent(t, w.Events(), cfg.SettleWindow+150*time.Millisecond)
}

func TestWatcher_NoDuplicateWatchRegistration(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 2
	cfg.SettleWindow = 30 * time.Millisecond
	cfg.MaxWait = 2 * time.Second

	w, src := startTestWatcher(t, root, cfg)

	albumDir := filepath.Join(root, "Artist", "Album")
	mkdirReal(t, albumDir)
	trackPath := filepath.Join(albumDir, "01.flac")
	writeReal(t, trackPath, "data")

	// Emit overlapping Create notifications for the same directory, as a
	// real OS might under rapid mkdir+populate — must not register more
	// than one watch for the same path.
	src.Emit(fswatch.RawEvent{Path: filepath.Join(root, "Artist"), Op: fswatch.OpCreate})
	src.Emit(fswatch.RawEvent{Path: albumDir, Op: fswatch.OpCreate})
	src.Emit(fswatch.RawEvent{Path: albumDir, Op: fswatch.OpCreate})
	src.Emit(fswatch.RawEvent{Path: trackPath, Op: fswatch.OpCreate})
	src.Emit(fswatch.RawEvent{Path: albumDir, Op: fswatch.OpCreate})

	waitForUnitEvent(t, w.Events(), 2*time.Second)

	if n := src.AddCalls(root); n != 1 {
		t.Fatalf("Add(%q) called %d times, want 1", root, n)
	}
	if n := src.AddCalls(albumDir); n != 1 {
		t.Fatalf("Add(%q) called %d times, want 1", albumDir, n)
	}
}

// TestWatcher_ConcurrentBursts stresses many units settling concurrently
// (simulating several albums copying in at once) to catch data races in
// the run loop's shared state and to confirm each unit fires exactly
// once, with the correct file set, despite events for every unit arriving
// interleaved from multiple goroutines. Run with -race.
func TestWatcher_ConcurrentBursts(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 1
	cfg.SettleWindow = 30 * time.Millisecond
	cfg.MaxWait = 3 * time.Second

	w, src := startTestWatcher(t, root, cfg)

	const units = 25
	const filesPerUnit = 8

	wantFiles := make(map[string]map[string]bool, units)
	var wg sync.WaitGroup
	for u := 0; u < units; u++ {
		unitDir := filepath.Join(root, fmt.Sprintf("Title-%02d", u))
		mkdirReal(t, unitDir)
		wantFiles[unitDir] = make(map[string]bool)

		wg.Add(1)
		go func(unitDir string) {
			defer wg.Done()
			src.Emit(fswatch.RawEvent{Path: unitDir, Op: fswatch.OpCreate})
			for f := 0; f < filesPerUnit; f++ {
				p := filepath.Join(unitDir, fmt.Sprintf("file-%d", f))
				writeReal(t, p, "x")
				src.Emit(fswatch.RawEvent{Path: p, Op: fswatch.OpCreate})
			}
		}(unitDir)
	}
	wg.Wait()

	seen := make(map[string]fswatch.Event, units)
	deadline := time.After(5 * time.Second)
	for len(seen) < units {
		select {
		case ev := <-w.Events():
			if prior, dup := seen[ev.Path]; dup {
				t.Fatalf("unit %q fired more than once: first %+v, second %+v", ev.Path, prior, ev)
			}
			seen[ev.Path] = ev
		case <-deadline:
			t.Fatalf("timed out with %d/%d units settled", len(seen), units)
		}
	}

	for unitDir := range wantFiles {
		ev, ok := seen[unitDir]
		if !ok {
			t.Fatalf("unit %q never settled", unitDir)
		}
		if len(ev.Files) != filesPerUnit {
			t.Fatalf("unit %q Files = %v, want %d entries", unitDir, ev.Files, filesPerUnit)
		}
		dedup := make(map[string]bool, len(ev.Files))
		for _, f := range ev.Files {
			if dedup[f] {
				t.Fatalf("unit %q has duplicate file %q", unitDir, f)
			}
			dedup[f] = true
		}
	}
}

func TestWatcher_IgnoreGlobsExcludedFromFilesButStillResetTimer(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 1
	cfg.SettleWindow = 80 * time.Millisecond
	cfg.MaxWait = 2 * time.Second
	cfg.IgnoreGlobs = []string{"*.part"}

	w, src := startTestWatcher(t, root, cfg)

	unitDir := filepath.Join(root, "Title")
	mkdirReal(t, unitDir)
	src.Emit(fswatch.RawEvent{Path: unitDir, Op: fswatch.OpCreate})

	realFile := filepath.Join(unitDir, "video.mp4")
	writeReal(t, realFile, "data")
	src.Emit(fswatch.RawEvent{Path: realFile, Op: fswatch.OpCreate})

	start := time.Now()
	// Trickle an ignored file in just before the window would otherwise
	// close, proving it still counts as activity.
	time.Sleep(cfg.SettleWindow - 20*time.Millisecond)
	ignoredFile := filepath.Join(unitDir, "video.mp4.part")
	writeReal(t, ignoredFile, "partial")
	src.Emit(fswatch.RawEvent{Path: ignoredFile, Op: fswatch.OpCreate})

	ev := waitForUnitEvent(t, w.Events(), 2*time.Second)
	elapsed := time.Since(start)

	if elapsed < cfg.SettleWindow {
		t.Fatalf("settled after %s, before the extended SettleWindow (%s) — ignored file did not reset the timer", elapsed, cfg.SettleWindow)
	}
	for _, f := range ev.Files {
		if f == ignoredFile {
			t.Fatalf("Files = %v, must not include ignored path %q", ev.Files, ignoredFile)
		}
	}
	if len(ev.Files) != 1 || ev.Files[0] != realFile {
		t.Fatalf("Files = %v, want [%q]", ev.Files, realFile)
	}
}

func TestWatcher_SecondBurstAfterSettleIsUpdated(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 1
	cfg.SettleWindow = 30 * time.Millisecond
	cfg.MaxWait = 2 * time.Second

	w, src := startTestWatcher(t, root, cfg)

	unitDir := filepath.Join(root, "Title")
	mkdirReal(t, unitDir)
	src.Emit(fswatch.RawEvent{Path: unitDir, Op: fswatch.OpCreate})

	first := waitForUnitEvent(t, w.Events(), 2*time.Second)
	if first.Kind != fswatch.Created {
		t.Fatalf("first Kind = %v, want Created", first.Kind)
	}

	newFile := filepath.Join(unitDir, "extra.mp4")
	writeReal(t, newFile, "data")
	src.Emit(fswatch.RawEvent{Path: newFile, Op: fswatch.OpCreate})

	second := waitForUnitEvent(t, w.Events(), 2*time.Second)
	if second.Kind != fswatch.Updated {
		t.Fatalf("second Kind = %v, want Updated", second.Kind)
	}
}

func TestNew_Validation(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name   string
		source fswatch.Source
		roots  []string
		cfg    fswatch.Config
	}{
		{"nil source", nil, []string{root}, fswatch.DefaultConfig()},
		{"no roots", fswatchtest.NewFake(), nil, fswatch.DefaultConfig()},
		{"invalid config", fswatchtest.NewFake(), []string{root}, fswatch.Config{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := fswatch.New(tt.source, tt.roots, tt.cfg); err == nil {
				t.Fatal("New did not return an error")
			}
		})
	}
}

func TestWatcher_WithResolverOverridesCoalesceDepth(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 5 // would be ignored: WithResolver takes priority
	cfg.SettleWindow = 30 * time.Millisecond
	cfg.MaxWait = 2 * time.Second

	src := fswatchtest.NewFake()
	w, err := fswatch.New(src, []string{root}, cfg, fswatch.WithResolver(fswatch.DepthResolver{Depth: 1}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = w.Close()
	})

	unitDir := filepath.Join(root, "Title")
	mkdirReal(t, unitDir)
	src.Emit(fswatch.RawEvent{Path: unitDir, Op: fswatch.OpCreate})

	ev := waitForUnitEvent(t, w.Events(), 2*time.Second)
	if ev.Path != unitDir {
		t.Fatalf("Path = %q, want %q (Depth=1 resolver should have been used, not CoalesceDepth=5)", ev.Path, unitDir)
	}
}

func TestWatcher_StartRejectsInvalidRoots(t *testing.T) {
	cfg := fswatch.DefaultConfig()

	t.Run("missing root", func(t *testing.T) {
		src := fswatchtest.NewFake()
		w, err := fswatch.New(src, []string{filepath.Join(t.TempDir(), "missing")}, cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if err := w.Start(context.Background()); err == nil {
			t.Fatal("Start did not return an error for a missing root")
		}
	})

	t.Run("root is a file, not a directory", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "file.txt")
		writeReal(t, root, "data")
		src := fswatchtest.NewFake()
		w, err := fswatch.New(src, []string{root}, cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if err := w.Start(context.Background()); err == nil {
			t.Fatal("Start did not return an error for a root that is a file")
		}
	})
}

func TestWatcher_AddFailureSurfacesOnErrorsChannel(t *testing.T) {
	root := t.TempDir()
	cfg := fswatch.DefaultConfig()
	cfg.CoalesceDepth = 1
	cfg.SettleWindow = 30 * time.Millisecond
	cfg.MaxWait = 2 * time.Second

	src := fswatchtest.NewFake()
	failDir := filepath.Join(root, "Title")
	mkdirReal(t, failDir)
	src.FailAdd(failDir, fmt.Errorf("simulated watch failure"))

	w, err := fswatch.New(src, []string{root}, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = w.Close()
	})

	select {
	case err := <-w.Errors():
		if err == nil {
			t.Fatal("received nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the watch-registration failure on Errors()")
	}
}
