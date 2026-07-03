package fs

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type watcher struct {
	debounce time.Duration
	extMap   map[string]domain.ContentType
}

// NewWatcher returns a FileWatcher backed by fsnotify. debounce controls how
// long the adapter waits after the last Write event before emitting — needed
// because large file copies produce many intermediate Write events.
func NewWatcher(debounce time.Duration) ports.FileWatcher {
	return &watcher{
		debounce: debounce,
		extMap:   buildExtMap(),
	}
}

var _ ports.FileWatcher = (*watcher)(nil)

func (w *watcher) Watch(ctx context.Context, roots []string) (<-chan ports.WatchEvent, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	for _, root := range roots {
		if err := addRecursive(fw, root); err != nil {
			_ = fw.Close()
			return nil, fmt.Errorf("watch root %s: %w", root, err)
		}
	}

	ch := make(chan ports.WatchEvent)
	go w.run(ctx, fw, ch)
	return ch, nil
}

// addRecursive walks root and adds every directory to the watcher.
// fsnotify v1.9 does not expose WithRecurse as a stable API; we implement
// recursion here and augment it at runtime for newly created subdirectories.
func addRecursive(fw *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("watcher: directory entry error", "path", path, "err", err)
			return nil //nolint:nilerr // skip unreadable entries without aborting the walk
		}
		if d.IsDir() {
			if addErr := fw.Add(path); addErr != nil {
				slog.Warn("watcher: could not watch directory", "path", path, "err", addErr)
			}
		}
		return nil
	})
}

// pendingEntry tracks the debounce timer and the earliest event op for a path.
type pendingEntry struct {
	timer *time.Timer
	op    ports.WatchOp
}

func (w *watcher) run(ctx context.Context, fw *fsnotify.Watcher, ch chan<- ports.WatchEvent) {
	defer close(ch)
	defer func() { _ = fw.Close() }()

	var mu sync.Mutex
	timers := make(map[string]*pendingEntry)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-fw.Events:
			if !ok {
				return
			}
			w.handleEvent(ctx, fw, event, ch, &mu, timers)
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			slog.WarnContext(ctx, "fsnotify error", "err", err)
		}
	}
}

func (w *watcher) handleEvent(
	ctx context.Context,
	fw *fsnotify.Watcher,
	event fsnotify.Event,
	ch chan<- ports.WatchEvent,
	mu *sync.Mutex,
	timers map[string]*pendingEntry,
) {
	// Newly created directories are added to the watcher so their contents
	// receive events too.
	if event.Has(fsnotify.Create) {
		if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
			if err := addRecursive(fw, event.Name); err != nil {
				slog.Warn("watcher: failed to watch new directory", "path", event.Name, "err", err)
			}
			return
		}
	}

	if _, ok := w.extMap[filepath.Ext(event.Name)]; !ok {
		return
	}

	var eventOp ports.WatchOp
	switch {
	case event.Has(fsnotify.Remove):
		eventOp = ports.WatchRemoved
	case event.Has(fsnotify.Create):
		eventOp = ports.WatchCreated
	case event.Has(fsnotify.Write):
		eventOp = ports.WatchModified
	default:
		return
	}

	path := event.Name
	mu.Lock()
	if entry, exists := timers[path]; exists {
		entry.timer.Stop()
		// Preserve WatchCreated if it was the first event; Create beats Write.
		if eventOp == ports.WatchCreated {
			entry.op = ports.WatchCreated
		}
		entry.timer = time.AfterFunc(w.debounce, func() {
			w.fire(ctx, ch, path, mu, timers)
		})
		mu.Unlock()
		return
	}
	p := &pendingEntry{op: eventOp}
	p.timer = time.AfterFunc(w.debounce, func() {
		w.fire(ctx, ch, path, mu, timers)
	})
	timers[path] = p
	mu.Unlock()
}

func (w *watcher) fire(ctx context.Context, ch chan<- ports.WatchEvent, path string, mu *sync.Mutex, timers map[string]*pendingEntry) {
	mu.Lock()
	entry, ok := timers[path]
	if !ok {
		mu.Unlock()
		return
	}
	op := entry.op
	delete(timers, path)
	mu.Unlock()

	if _, err := os.Stat(path); err != nil {
		op = ports.WatchRemoved
	}

	ct, ok := w.extMap[filepath.Ext(path)]
	if !ok {
		return
	}

	select {
	case ch <- ports.WatchEvent{Path: path, ContentType: ct, Op: op}:
	case <-ctx.Done():
	}
}
