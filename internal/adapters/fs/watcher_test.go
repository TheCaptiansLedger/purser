package fs_test

import (
	"context"
	"os"
	"path/filepath"
	"purser/internal/adapters/fs"
	"purser/internal/ports"
	"testing"
	"time"
)

const watchDebounce = 100 * time.Millisecond

func TestWatcher_EmitsEventForMediaFile(t *testing.T) {
	dir := t.TempDir()

	w := fs.NewWatcherNoStabilityCheck(watchDebounce)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := w.Watch(ctx, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	// Give the watcher time to initialise before writing.
	time.Sleep(50 * time.Millisecond)

	filePath := filepath.Join(dir, "track.flac")
	if err := os.WriteFile(filePath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-ch:
		if event.Path != filePath {
			t.Errorf("path = %q, want %q", event.Path, filePath)
		}
		if event.Op != ports.WatchCreated && event.Op != ports.WatchModified {
			t.Errorf("op = %d, want WatchCreated(%d) or WatchModified(%d)", event.Op, ports.WatchCreated, ports.WatchModified)
		}
	case <-ctx.Done():
		t.Fatal("timeout: no watch event received for .flac file")
	}
}

func TestWatcher_SkipsNonMediaExtension(t *testing.T) {
	dir := t.TempDir()

	w := fs.NewWatcherNoStabilityCheck(watchDebounce)
	// Short timeout: we expect no event.
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	ch, err := w.Watch(ctx, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-ch:
		t.Errorf("unexpected event for non-media file: path=%q op=%d", event.Path, event.Op)
	case <-ctx.Done():
		// expected: no event emitted for .txt
	}
}

// TestWatcher_DroppedDirectoryEmitsEventsForExistingFiles verifies that when a
// directory containing media files is moved/copied into a watched root, events
// are emitted for files that were already inside the directory at creation time.
// This covers the macOS behaviour where the OS emits a single Create for the
// directory but no individual Create events for pre-existing contents.
func TestWatcher_DroppedDirectoryEmitsEventsForExistingFiles(t *testing.T) {
	root := t.TempDir()

	// Pre-populate a staging area (outside the watched root) with FLAC files.
	staging := t.TempDir()
	files := []string{"track01.flac", "track02.flac", "cover.jpg"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(staging, f), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	w := fs.NewWatcherNoStabilityCheck(watchDebounce)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := w.Watch(ctx, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	// Move the staging directory into the watched root — simulates "drop folder".
	dest := filepath.Join(root, "album")
	if err := os.Rename(staging, dest); err != nil {
		t.Fatal(err)
	}

	// Collect events until we have the expected count or timeout.
	got := map[string]bool{}
	deadline := time.After(5 * time.Second)
collect:
	for len(got) < 2 {
		select {
		case event := <-ch:
			got[filepath.Base(event.Path)] = true
		case <-deadline:
			break collect
		}
	}

	// We expect events for the two .flac files; .jpg is not a media extension.
	if !got["track01.flac"] {
		t.Error("no event for track01.flac")
	}
	if !got["track02.flac"] {
		t.Error("no event for track02.flac")
	}
	if got["cover.jpg"] {
		t.Error("unexpected event for non-media cover.jpg")
	}
}

// TestWatcher_DroppedDirectory_UppercaseExtension verifies that files with
// uppercase extensions (.FLAC, .MP3) are treated the same as lowercase (.flac).
func TestWatcher_DroppedDirectory_UppercaseExtension(t *testing.T) {
	root := t.TempDir()
	staging := t.TempDir()
	if err := os.WriteFile(filepath.Join(staging, "track.FLAC"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := fs.NewWatcherNoStabilityCheck(watchDebounce)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := w.Watch(ctx, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	dest := filepath.Join(root, "album")
	if err := os.Rename(staging, dest); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-ch:
		if filepath.Base(event.Path) != "track.FLAC" {
			t.Errorf("unexpected path %q", event.Path)
		}
	case <-ctx.Done():
		t.Fatal("timeout: no event for uppercase-extension file")
	}
}

// TestWatcher_DroppedDirectory_DeferredScan simulates VirtioFS / Docker: the
// directory arrives empty (Create event fires before files are present), and
// files appear shortly after. The watcher must still emit events for them.
func TestWatcher_DroppedDirectory_DeferredScan(t *testing.T) {
	root := t.TempDir()

	w := fs.NewWatcherNoStabilityCheck(watchDebounce)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ch, err := w.Watch(ctx, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	// Create the directory empty first — this triggers the watcher.
	dir := filepath.Join(root, "album")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Simulate VirtioFS delay: write files *after* the directory event.
	time.Sleep(200 * time.Millisecond)
	for _, f := range []string{"track01.flac", "track02.flac"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := map[string]bool{}
	deadline := time.After(10 * time.Second)
	for len(got) < 2 {
		select {
		case event := <-ch:
			got[filepath.Base(event.Path)] = true
		case <-deadline:
			t.Fatalf("timeout: only received %v, want track01.flac and track02.flac", got)
		}
	}
}

func TestWatcher_DebounceCoalescesEvents(t *testing.T) {
	dir := t.TempDir()

	w := fs.NewWatcherNoStabilityCheck(watchDebounce)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := w.Watch(ctx, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	filePath := filepath.Join(dir, "album.mp3")
	// Write the file multiple times in quick succession to trigger many events.
	for i := range 5 {
		if err := os.WriteFile(filePath, []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Collect events for slightly longer than the debounce window.
	deadline := time.After(watchDebounce * 4)
	var events []ports.WatchEvent
loop:
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				break loop
			}
			events = append(events, event)
		case <-deadline:
			break loop
		case <-ctx.Done():
			break loop
		}
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if len(events) > 2 {
		t.Errorf("debounce produced %d events, want ≤2 (coalesced)", len(events))
	}
}
