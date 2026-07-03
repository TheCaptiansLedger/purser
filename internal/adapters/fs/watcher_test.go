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

	w := fs.NewWatcher(watchDebounce)
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

	w := fs.NewWatcher(watchDebounce)
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

func TestWatcher_DebounceCoalescesEvents(t *testing.T) {
	dir := t.TempDir()

	w := fs.NewWatcher(watchDebounce)
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
