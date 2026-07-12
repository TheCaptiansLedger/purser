// Package fsnotify is the fsnotify-backed fswatch.Source adapter — a thin
// translation layer over github.com/fsnotify/fsnotify. All recursion,
// debouncing, and unit resolution lives in pkg/fswatch itself; this
// package only forwards raw per-path events.
package fsnotify

import (
	"fmt"
	"log/slog"
	"purser/pkg/fswatch"
	"sync"

	notify "github.com/fsnotify/fsnotify"
)

// Watcher is the fswatch.Source implementation backed by fsnotify.
type Watcher struct {
	inner  *notify.Watcher
	logger *slog.Logger

	events chan fswatch.RawEvent
	errors chan error
	done   chan struct{}

	closeOnce sync.Once
	wg        sync.WaitGroup
}

// New constructs an fsnotify-backed fswatch.Source.
func New(opts ...Option) (*Watcher, error) {
	inner, err := notify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fswatch/fsnotify: creating watcher: %w", err)
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	w := &Watcher{
		inner:  inner,
		logger: o.logger.With("component", "fswatch.fsnotify"),
		events: make(chan fswatch.RawEvent),
		errors: make(chan error),
		done:   make(chan struct{}),
	}

	w.wg.Add(1)
	go w.pump()
	return w, nil
}

func (w *Watcher) pump() {
	defer w.wg.Done()
	defer close(w.events)
	defer close(w.errors)

	for {
		select {
		case ev, ok := <-w.inner.Events:
			if !ok {
				return
			}
			raw, ok := translate(ev)
			if !ok {
				continue
			}
			select {
			case w.events <- raw:
			case <-w.done:
				return
			}
		case err, ok := <-w.inner.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			case <-w.done:
				return
			}
		case <-w.done:
			return
		}
	}
}

func translate(ev notify.Event) (fswatch.RawEvent, bool) {
	switch {
	case ev.Has(notify.Create):
		return fswatch.RawEvent{Path: ev.Name, Op: fswatch.OpCreate}, true
	case ev.Has(notify.Write):
		return fswatch.RawEvent{Path: ev.Name, Op: fswatch.OpWrite}, true
	case ev.Has(notify.Remove):
		return fswatch.RawEvent{Path: ev.Name, Op: fswatch.OpRemove}, true
	case ev.Has(notify.Rename):
		return fswatch.RawEvent{Path: ev.Name, Op: fswatch.OpRename}, true
	default:
		// Chmod-only events carry no information relevant to settling.
		return fswatch.RawEvent{}, false
	}
}

// Add implements fswatch.Source.
func (w *Watcher) Add(path string) error { return w.inner.Add(path) }

// Remove implements fswatch.Source.
func (w *Watcher) Remove(path string) error { return w.inner.Remove(path) }

// Events implements fswatch.Source.
func (w *Watcher) Events() <-chan fswatch.RawEvent { return w.events }

// Errors implements fswatch.Source.
func (w *Watcher) Errors() <-chan error { return w.errors }

// Close implements fswatch.Source.
func (w *Watcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		err = w.inner.Close()
		w.wg.Wait()
		w.logger.Debug("closed")
	})
	return err
}
