package fswatchtest

import (
	"os"
	"path/filepath"
	"purser/pkg/fswatch"
	"testing"
	"time"
)

// NewSourceFunc returns a fresh Source for the duration of a single
// subtest. t.Cleanup is used by the contract to close it.
type NewSourceFunc func(t *testing.T) fswatch.Source

// TestSource runs the shared Source contract against newSource. Each check
// is its own top-level subtest so a single failure identifies exactly
// which part of the contract broke. Every real filesystem operation uses
// t.TempDir(), so this requires no network access and is CI-safe.
func TestSource(t *testing.T, newSource NewSourceFunc) {
	t.Helper()

	t.Run("create observed for a watched directory", func(t *testing.T) { testCreateObserved(t, newSource) })
	t.Run("write observed for a watched file", func(t *testing.T) { testWriteObserved(t, newSource) })
	t.Run("remove observed for a watched path", func(t *testing.T) { testRemoveObserved(t, newSource) })
	t.Run("events stop after Remove", func(t *testing.T) { testEventsStopAfterRemove(t, newSource) })
	t.Run("Add on a missing path returns an error", func(t *testing.T) { testAddMissingPath(t, newSource) })
}

func waitForEvent(t *testing.T, ch <-chan fswatch.RawEvent, want string, timeout time.Duration) fswatch.RawEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if filepath.Clean(ev.Path) == filepath.Clean(want) {
				return ev
			}
			// A different path (e.g. a directory's own mtime bump) —
			// keep waiting for the one we care about.
		case <-deadline:
			t.Fatalf("timed out waiting for an event on %q", want)
		}
	}
}

func testCreateObserved(t *testing.T, newSource NewSourceFunc) {
	dir := t.TempDir()
	src := newSource(t)

	if err := src.Add(dir); err != nil {
		t.Fatalf("Add(%q) returned error: %v", dir, err)
	}

	target := filepath.Join(dir, "child")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	ev := waitForEvent(t, src.Events(), target, 5*time.Second)
	if ev.Op != fswatch.OpCreate {
		t.Fatalf("Op = %v, want OpCreate", ev.Op)
	}
}

func testWriteObserved(t *testing.T, newSource NewSourceFunc) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(target, []byte("initial"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	src := newSource(t)
	if err := src.Add(dir); err != nil {
		t.Fatalf("Add(%q) returned error: %v", dir, err)
	}

	if err := os.WriteFile(target, []byte("updated"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ev := waitForEvent(t, src.Events(), target, 5*time.Second)
	if ev.Op != fswatch.OpWrite && ev.Op != fswatch.OpCreate {
		t.Fatalf("Op = %v, want OpWrite (or OpCreate, platform-dependent for truncate+write)", ev.Op)
	}
}

func testRemoveObserved(t *testing.T, newSource NewSourceFunc) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	src := newSource(t)
	if err := src.Add(dir); err != nil {
		t.Fatalf("Add(%q) returned error: %v", dir, err)
	}

	if err := os.Remove(target); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	ev := waitForEvent(t, src.Events(), target, 5*time.Second)
	if ev.Op != fswatch.OpRemove {
		t.Fatalf("Op = %v, want OpRemove", ev.Op)
	}
}

func testEventsStopAfterRemove(t *testing.T, newSource NewSourceFunc) {
	dir := t.TempDir()
	src := newSource(t)
	if err := src.Add(dir); err != nil {
		t.Fatalf("Add(%q) returned error: %v", dir, err)
	}
	if err := src.Remove(dir); err != nil {
		t.Fatalf("Remove(%q) returned error: %v", dir, err)
	}

	target := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	select {
	case ev := <-src.Events():
		t.Fatalf("received event %+v for a path removed before the write", ev)
	case <-time.After(500 * time.Millisecond):
		// expected: no event
	}
}

func testAddMissingPath(t *testing.T, newSource NewSourceFunc) {
	src := newSource(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := src.Add(missing); err == nil {
		t.Fatal("Add on a missing path did not return an error")
	}
}
