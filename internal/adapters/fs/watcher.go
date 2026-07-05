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
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type watcher struct {
	debounce       time.Duration
	stabilityCheck time.Duration
	extMap         map[string]domain.ContentType
}

// NewWatcher returns a FileWatcher backed by fsnotify. debounce controls how
// long the adapter waits after the last Write event before emitting — needed
// because large file copies produce many intermediate Write events. A 1-second
// size-stability check is applied before emitting to guard against files that
// are still being written when the debounce fires.
func NewWatcher(debounce time.Duration) ports.FileWatcher {
	return &watcher{
		debounce:       debounce,
		stabilityCheck: time.Second,
		extMap:         buildExtMap(),
	}
}

// NewWatcherNoStabilityCheck returns a watcher with the stability check
// disabled. Use in tests that write files synchronously and do not need the
// extra delay that guards against in-progress network copies.
func NewWatcherNoStabilityCheck(debounce time.Duration) ports.FileWatcher {
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
		slog.InfoContext(ctx, "watcher: watching root", "path", root)
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
	// receive events too. When a directory is dropped/moved in (e.g. dropping a
	// folder of FLAC files), the OS emits a single Create for the directory but
	// no individual Create events for files already inside it. Walk the new
	// directory in a background goroutine and emit WatchCreated for any media
	// files already present. The goroutine is necessary because ch is unbuffered
	// and the consumer (processFile) is slow; blocking here would stall the
	// fsnotify event loop, causing it to drop subsequent events.
	if event.Has(fsnotify.Create) {
		if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
			slog.InfoContext(ctx, "watcher: new directory detected, scanning for existing files", "path", event.Name)
			if err := addRecursive(fw, event.Name); err != nil {
				slog.Warn("watcher: failed to watch new directory", "path", event.Name, "err", err)
			}
			go w.emitExistingFiles(ctx, event.Name, ch)
			return
		}
	}

	if _, ok := w.extMap[strings.ToLower(filepath.Ext(event.Name))]; !ok {
		return
	}
	if strings.HasPrefix(filepath.Base(event.Name), "._") {
		return
	}
	slog.InfoContext(ctx, "watcher: media file event", "path", event.Name, "op", event.Op.String())

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

// emitExistingFiles walks dir and sends a WatchCreated event for every media
// file already present. Called in a goroutine when a new directory is detected
// so that the fsnotify event loop is never blocked on channel sends.
//
// Docker on macOS (VirtioFS): a directory rename arrives as a single Create on
// the parent; VirtioFS may write the directory files in as separate events
// *after* the parent Create. If the first walk finds nothing, we retry after
// a short delay so copy-in files are still caught.
func (w *watcher) emitExistingFiles(ctx context.Context, dir string, ch chan<- ports.WatchEvent) {
	count := w.walkAndEmit(ctx, dir, ch)
	slog.InfoContext(ctx, "watcher: finished scanning new directory", "dir", dir, "files_emitted", count)

	if count == 0 {
		// The directory was empty at scan time. This is normal for copy-in
		// (VirtioFS, cp from another volume): the parent gets a Create for the
		// empty directory, then individual file events follow. Those events will
		// be handled by the normal debounce path. Do one delayed retry in case
		// VirtioFS delays the per-file events far enough that they arrive during
		// a brief gap before the watch is fully registered.
		select {
		case <-time.After(750 * time.Millisecond):
		case <-ctx.Done():
			return
		}
		count = w.walkAndEmit(ctx, dir, ch)
		slog.InfoContext(ctx, "watcher: deferred scan of new directory", "dir", dir, "files_emitted", count)
	}
}

// walkAndEmit performs a single recursive walk of dir, emitting a WatchCreated
// event for each media file. It returns the number of events emitted.
func (w *watcher) walkAndEmit(ctx context.Context, dir string, ch chan<- ports.WatchEvent) int {
	var count, skippedExt, skippedHidden int
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.WarnContext(ctx, "watcher: scan: entry error", "path", path, "err", err)
			return nil //nolint:nilerr // skip unreadable entries; walk must continue
		}
		if d.IsDir() {
			return nil
		}
		// Normalise to lowercase so ".FLAC" matches ".flac" in extMap.
		if _, ok := w.extMap[strings.ToLower(filepath.Ext(path))]; !ok {
			slog.DebugContext(ctx, "watcher: scan: skipping (non-media extension)", "path", path, "ext", filepath.Ext(path))
			skippedExt++
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), "._") {
			skippedHidden++
			return nil
		}
		fi, statErr := os.Stat(path)
		if statErr != nil {
			slog.WarnContext(ctx, "watcher: scan: stat failed", "path", path, "err", statErr)
			return nil //nolint:nilerr // file may have vanished between walk and stat
		}
		slog.InfoContext(ctx, "watcher: emitting event for pre-existing file", "path", path)
		select {
		case ch <- ports.WatchEvent{Path: path, Size: fi.Size(), Op: ports.WatchCreated}:
			count++
		case <-ctx.Done():
			return filepath.SkipAll
		}
		return nil
	})
	if skippedExt > 0 {
		slog.InfoContext(ctx, "watcher: scan: skipped files with non-media extensions", "dir", dir, "count", skippedExt)
	}
	return count
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

	var size int64
	if fi, err := os.Stat(path); err != nil {
		op = ports.WatchRemoved
	} else {
		size = fi.Size()
	}

	// Stability check: for write/create events, wait and re-stat. If the size
	// changed, the file is still being written (slow network copy, etc.). The
	// ongoing writes will have re-armed the debounce, so we can safely drop this
	// firing and let that new debounce handle it.
	if op != ports.WatchRemoved && w.stabilityCheck > 0 {
		select {
		case <-time.After(w.stabilityCheck):
		case <-ctx.Done():
			return
		}
		fi2, err := os.Stat(path)
		if err != nil {
			op = ports.WatchRemoved
		} else if fi2.Size() != size {
			return
		} else {
			size = fi2.Size()
		}
	}

	if _, ok := w.extMap[strings.ToLower(filepath.Ext(path))]; !ok {
		return
	}

	select {
	case ch <- ports.WatchEvent{Path: path, Size: size, Op: op}:
	case <-ctx.Done():
	}
}
