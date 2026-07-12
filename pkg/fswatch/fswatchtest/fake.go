// Package fswatchtest provides a fake fswatch.Source for engine unit
// tests and the shared contract test suite run against every real Source
// adapter (see ADR 0003's contract-test convention). It is a normal
// buildable package, not a _test.go file, because Go test files cannot be
// imported across packages.
package fswatchtest

import (
	"fmt"
	"purser/pkg/fswatch"
	"sync"
)

// Fake is an in-memory fswatch.Source for tests. Emit/EmitError inject raw
// events as if the OS had reported them. Safe for concurrent use: Emit may
// be called from multiple goroutines to simulate concurrent bursts of
// activity, matching how Watcher itself is exercised under `go test -race`.
type Fake struct {
	mu      sync.Mutex
	watched map[string]int // path -> Add call count; catches duplicate Add
	failAdd map[string]error
	closed  bool

	events chan fswatch.RawEvent
	errs   chan error
}

// NewFake returns a ready-to-use Fake. Buffers are generous so tests can
// Emit bursts without a reader draining concurrently.
func NewFake() *Fake {
	return &Fake{
		watched: make(map[string]int),
		events:  make(chan fswatch.RawEvent, 4096),
		errs:    make(chan error, 256),
	}
}

// Add implements fswatch.Source.
func (f *Fake) Add(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return fmt.Errorf("fswatchtest: fake is closed")
	}
	if err, fail := f.failAdd[path]; fail {
		return err
	}
	f.watched[path]++
	return nil
}

// FailAdd makes a future Add(path) call return err instead of succeeding —
// tests use this to exercise watch-registration failure handling.
func (f *Fake) FailAdd(path string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAdd == nil {
		f.failAdd = make(map[string]error)
	}
	f.failAdd[path] = err
}

// Remove implements fswatch.Source.
func (f *Fake) Remove(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.watched, path)
	return nil
}

// Events implements fswatch.Source.
func (f *Fake) Events() <-chan fswatch.RawEvent { return f.events }

// Errors implements fswatch.Source.
func (f *Fake) Errors() <-chan error { return f.errs }

// Close implements fswatch.Source.
func (f *Fake) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	close(f.events)
	close(f.errs)
	return nil
}

// Emit pushes a raw event as if the OS had reported it. Must not be called
// after Close.
func (f *Fake) Emit(ev fswatch.RawEvent) { f.events <- ev }

// EmitError pushes an asynchronous error. Must not be called after Close.
func (f *Fake) EmitError(err error) { f.errs <- err }

// AddCalls reports how many times Add was called for path — tests use
// this to assert a directory is never watched twice.
func (f *Fake) AddCalls(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watched[path]
}

// WatchedPaths returns the currently watched paths (Added without a
// matching Remove).
func (f *Fake) WatchedPaths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	paths := make([]string, 0, len(f.watched))
	for p := range f.watched {
		paths = append(paths, p)
	}
	return paths
}
