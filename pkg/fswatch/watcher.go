// Package fswatch watches one or more root directories, recursively and
// intelligently: it discovers new subdirectories and watches them as they
// appear, and debounces bursts of raw filesystem activity into a single
// settled Event per "unit" (a caller-defined directory boundary, e.g. an
// Album directory rather than each track file inside it — see
// UnitResolver) so a consumer never sees a directory mid-write or a track
// reported as if it were a standalone album.
//
// The raw OS notification mechanism is a swappable Source port (fsnotify
// today; see pkg/fswatch/fsnotify). The debounce/coalesce engine in this
// file is not swappable — it's the same logic regardless of backend — so
// it's a concrete Watcher built on top of Source, not itself a port.
package fswatch

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// pendingUnit tracks in-flight, not-yet-settled activity for a single
// unit. gen is bumped on every touch and captured by the scheduled timer's
// closure, so a timer signal for a superseded generation (the unit fired,
// or was replaced by a new pending entry, before the old timer's goroutine
// got to run) is recognized and dropped instead of firing a duplicate or
// stale Event.
type pendingUnit struct {
	root      string
	kind      EventKind
	files     map[string]struct{}
	firstSeen time.Time
	gen       uint64
	timer     *time.Timer
}

type timerSignal struct {
	unit string
	gen  uint64
}

// Watcher is the orchestration engine built on top of a Source. Construct
// with New, call Start once, read Events (and Errors) until Close.
type Watcher struct {
	roots    []string
	cfg      Config
	resolver UnitResolver
	source   Source

	logger *slog.Logger
	tracer trace.Tracer

	eventsRawCounter     metric.Int64Counter
	eventsEmittedCounter metric.Int64Counter
	watchesActiveCounter metric.Int64UpDownCounter
	settleDuration       metric.Float64Histogram

	events chan Event
	errors chan error

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup // the run() goroutine
	deliverWG sync.WaitGroup // timer-fired and event/error delivery goroutines

	timerFired chan timerSignal

	// Everything below is owned exclusively by the run() goroutine — it
	// is never touched from any other goroutine, so none of it needs a
	// mutex and none of it can race. Delivery to Events()/Errors() is
	// handed off to short-lived goroutines (see deliver/deliverError)
	// specifically so a slow consumer blocks only its own unit's
	// delivery, never this state or other units' timers.
	watched map[string]struct{}
	pending map[string]*pendingUnit
	known   map[string]struct{}
}

// New constructs a Watcher over roots, backed by source. source is
// required — Watcher has no default backend (mirrors pkg/cache: the port
// doesn't construct its own adapter). Typical use:
//
//	src, err := fsnotify.New()
//	w, err := fswatch.New(src, roots, fswatch.DefaultConfig())
func New(source Source, roots []string, cfg Config, opts ...Option) (*Watcher, error) {
	if source == nil {
		return nil, fmt.Errorf("fswatch: source must not be nil")
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("fswatch: at least one root is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("fswatch: invalid config: %w", err)
	}

	cleanRoots := make([]string, 0, len(roots))
	for _, r := range roots {
		abs, err := filepath.Abs(r)
		if err != nil {
			return nil, fmt.Errorf("fswatch: resolving root %q: %w", r, err)
		}
		cleanRoots = append(cleanRoots, filepath.Clean(abs))
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	resolver := o.resolver
	if resolver == nil {
		resolver = DepthResolver{Depth: cfg.CoalesceDepth}
	}

	w := &Watcher{
		roots:    cleanRoots,
		cfg:      cfg,
		resolver: resolver,
		source:   source,
		logger:   o.logger.With("component", "fswatch"),
		tracer:   o.tracerProvider.Tracer(instrumentationName),

		events: make(chan Event, cfg.EventBuffer),
		errors: make(chan error, 64),
		done:   make(chan struct{}),

		timerFired: make(chan timerSignal, 256),

		watched: make(map[string]struct{}),
		pending: make(map[string]*pendingUnit),
		known:   make(map[string]struct{}),
	}

	meter := o.meterProvider.Meter(instrumentationName)
	var err error
	if w.eventsRawCounter, err = meter.Int64Counter("fswatch.events.raw", metric.WithDescription("raw filesystem events observed")); err != nil {
		return nil, fmt.Errorf("fswatch: creating events.raw counter: %w", err)
	}
	if w.eventsEmittedCounter, err = meter.Int64Counter("fswatch.events.emitted", metric.WithDescription("settled unit events emitted")); err != nil {
		return nil, fmt.Errorf("fswatch: creating events.emitted counter: %w", err)
	}
	if w.watchesActiveCounter, err = meter.Int64UpDownCounter("fswatch.watches.active", metric.WithDescription("currently registered directory watches")); err != nil {
		return nil, fmt.Errorf("fswatch: creating watches.active counter: %w", err)
	}
	if w.settleDuration, err = meter.Float64Histogram("fswatch.settle.duration", metric.WithDescription("seconds from first activity to a unit settling"), metric.WithUnit("s")); err != nil {
		return nil, fmt.Errorf("fswatch: creating settle.duration histogram: %w", err)
	}

	return w, nil
}

// Start performs the initial recursive walk of every root (synchronously —
// Start does not return until every existing directory is registered with
// source) and then begins processing events in a background goroutine
// until ctx is done or Close is called.
func (w *Watcher) Start(ctx context.Context) error {
	for _, root := range w.roots {
		info, err := os.Stat(root)
		if err != nil {
			return fmt.Errorf("fswatch: root %q: %w", root, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("fswatch: root %q is not a directory", root)
		}
		w.registerSubtree(ctx, root, root)
	}

	w.wg.Add(1)
	go w.run(ctx)
	return nil
}

// Events returns the channel of settled unit events. Sends block if the
// consumer isn't draining fast enough (bounded by Config.EventBuffer) —
// losing a settle event would mean a caller silently never learns a
// directory changed, which is worse than backpressure.
func (w *Watcher) Events() <-chan Event { return w.events }

// Errors returns the channel of asynchronous errors (watch registration
// failures, Source-level errors).
func (w *Watcher) Errors() <-chan error { return w.errors }

// Close stops the run loop and closes source. Safe to call more than
// once; only the first call has effect. After Close returns, Events and
// Errors are closed.
func (w *Watcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		w.wg.Wait()
		err = w.source.Close()
	})
	return err
}

func (w *Watcher) run(ctx context.Context) {
	defer w.wg.Done()

	events := w.source.Events()
	errs := w.source.Errors()

	defer func() {
		for _, p := range w.pending {
			w.stopTimer(p)
		}
		w.deliverWG.Wait()
		close(w.events)
		close(w.errors)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case raw, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			w.handleRaw(ctx, raw)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			w.deliverError(ctx, err)
		case sig := <-w.timerFired:
			p, exists := w.pending[sig.unit]
			if !exists || p.gen != sig.gen {
				continue // stale signal: unit already fired, removed, or re-touched
			}
			w.fireUnit(ctx, sig.unit, p)
		}
	}
}

// registerSubtree walks dirPath (a subtree rooted at or under root),
// registering a watch for every directory found and seeding pending state
// for every entry — directories and files alike — via the same UnitResolver
// path normal events use. Called both for the initial per-root seed walk
// and, race-safely, whenever a Create event reports a new directory: the
// watch is added and the directory's pre-existing contents are folded into
// the same settle batch before control returns to the event loop, so files
// written in the gap between mkdir and watch registration are never missed.
func (w *Watcher) registerSubtree(ctx context.Context, root, dirPath string) {
	err := filepath.WalkDir(dirPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			w.deliverError(ctx, fmt.Errorf("fswatch: walking %s: %w", p, err))
			return nil
		}
		if d.IsDir() {
			w.addWatch(ctx, p)
		}
		if unit, ok := w.resolver.UnitFor(root, p); ok {
			// Directories (including the unit's own path) only reset the
			// settle timer; only real files belong in the reported Files
			// list.
			record := !d.IsDir() && !matchesAny(w.cfg.IgnoreGlobs, d.Name())
			w.touchUnit(ctx, root, unit, p, record)
		}
		return nil
	})
	if err != nil {
		w.deliverError(ctx, fmt.Errorf("fswatch: walking %s: %w", dirPath, err))
	}
}

func (w *Watcher) addWatch(ctx context.Context, path string) {
	if _, exists := w.watched[path]; exists {
		return
	}
	if err := w.source.Add(path); err != nil {
		w.deliverError(ctx, fmt.Errorf("fswatch: watching %s: %w", path, err))
		return
	}
	w.watched[path] = struct{}{}
	w.watchesActiveCounter.Add(ctx, 1)
	w.logger.DebugContext(ctx, "watch added", "path", path)
}

// pruneWatched removes every watched path equal to or nested under prefix
// (a unit/directory that no longer exists) — the filesystem is gone, so
// this is the only way to keep the in-memory watched set accurate; a
// best-effort Remove is issued to source for each in case the backend
// didn't already drop it on its own.
func (w *Watcher) pruneWatched(ctx context.Context, prefix string) {
	for p := range w.watched {
		if p == prefix || strings.HasPrefix(p, prefix+string(filepath.Separator)) {
			_ = w.source.Remove(p)
			delete(w.watched, p)
			w.watchesActiveCounter.Add(ctx, -1)
		}
	}
}

func (w *Watcher) handleRaw(ctx context.Context, raw RawEvent) {
	root, ok := w.rootFor(raw.Path)
	if !ok {
		return // stale event for a path no longer under any watched root
	}

	w.eventsRawCounter.Add(ctx, 1,
		metric.WithAttributes(attribute.String("root", root), attribute.String("op", raw.Op.String())))

	unit, ok := w.resolver.UnitFor(root, raw.Path)
	if !ok {
		return
	}

	// fsnotify reports Rename against the old path; the OS separately
	// generates a Create for the destination if it lands somewhere
	// watched. Treated identically to Remove for the old path.
	if (raw.Op == OpRemove || raw.Op == OpRename) && raw.Path == unit {
		w.handleUnitRemoved(ctx, root, unit)
		return
	}

	if raw.Op == OpCreate {
		if info, err := os.Stat(raw.Path); err == nil && info.IsDir() {
			w.registerSubtree(ctx, root, raw.Path)
			return
		}
	}

	record := !matchesAny(w.cfg.IgnoreGlobs, filepath.Base(raw.Path))
	w.touchUnit(ctx, root, unit, raw.Path, record)
}

// touchUnit marks unit as active, resetting its settle timer. changedPath
// is added to the unit's reported Files only when record is true — a
// directory (including the unit's own path) or an IgnoreGlobs match still
// counts as activity but must not appear in Files.
func (w *Watcher) touchUnit(ctx context.Context, root, unit, changedPath string, record bool) {
	now := time.Now()

	p, exists := w.pending[unit]
	if !exists {
		kind := Updated
		if _, known := w.known[unit]; !known {
			kind = Created
		}
		p = &pendingUnit{
			root:      root,
			kind:      kind,
			files:     make(map[string]struct{}),
			firstSeen: now,
		}
		w.pending[unit] = p
	}
	if record {
		p.files[changedPath] = struct{}{}
	}

	p.gen++

	fireAt := now.Add(w.cfg.SettleWindow)
	if maxFireAt := p.firstSeen.Add(w.cfg.MaxWait); fireAt.After(maxFireAt) {
		fireAt = maxFireAt
	}

	w.stopTimer(p)

	if delay := fireAt.Sub(now); delay > 0 {
		p.timer = w.scheduleTimer(unit, p.gen, delay)
		return
	}

	// MaxWait already reached: fire now instead of scheduling a
	// zero/negative-delay timer.
	w.fireUnit(ctx, unit, p)
}

func (w *Watcher) handleUnitRemoved(ctx context.Context, root, unit string) {
	if p, exists := w.pending[unit]; exists {
		w.stopTimer(p)
		delete(w.pending, unit)
	}
	delete(w.known, unit)
	w.pruneWatched(ctx, unit)

	ev := Event{Root: root, Path: unit, Kind: Removed, Time: time.Now()}
	w.logger.DebugContext(ctx, "unit removed", "root", root, "unit", unit)
	w.eventsEmittedCounter.Add(ctx, 1,
		metric.WithAttributes(attribute.String("root", root), attribute.String("kind", Removed.String())))
	w.deliver(ev)
}

// fireUnit finalizes and emits the Event for a settled (or MaxWait-expired)
// unit. Called only from run()'s goroutine, either synchronously from
// touchUnit (MaxWait already elapsed) or via a validated timerFired signal.
func (w *Watcher) fireUnit(ctx context.Context, unit string, p *pendingUnit) {
	ctx, span := w.tracer.Start(ctx, "fswatch.settle", trace.WithAttributes(
		attribute.String("root", p.root),
		attribute.String("unit", unit),
	))
	defer span.End()

	files := make([]string, 0, len(p.files))
	for f := range p.files {
		files = append(files, f)
	}
	sort.Strings(files)

	ev := Event{
		Root:  p.root,
		Path:  unit,
		Kind:  p.kind,
		Files: files,
		Time:  time.Now(),
	}
	span.SetAttributes(attribute.String("kind", p.kind.String()), attribute.Int("files", len(files)))

	if p.kind == Created {
		w.known[unit] = struct{}{}
	}
	delete(w.pending, unit)

	w.settleDuration.Record(ctx, time.Since(p.firstSeen).Seconds(),
		metric.WithAttributes(attribute.String("root", p.root)))
	w.eventsEmittedCounter.Add(ctx, 1,
		metric.WithAttributes(attribute.String("root", p.root), attribute.String("kind", p.kind.String())))
	w.logger.DebugContext(ctx, "unit settled", "root", p.root, "unit", unit, "kind", p.kind.String(), "files", len(files))

	w.deliver(ev)
}

func (w *Watcher) rootFor(path string) (string, bool) {
	best := ""
	for _, r := range w.roots {
		if path == r {
			return r, true
		}
		rel, err := filepath.Rel(r, path)
		if err != nil {
			continue
		}
		slashRel := filepath.ToSlash(rel)
		if slashRel == ".." || strings.HasPrefix(slashRel, "../") {
			continue
		}
		if len(r) > len(best) {
			best = r
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// scheduleTimer arms a timer that signals run() via timerFired once d
// elapses. The callback runs in its own goroutine (time.AfterFunc); it is
// tracked in deliverWG and selects on done so it can never leak or send on
// a channel nobody is reading during shutdown. gen is captured at schedule
// time so run() can recognize and drop a signal from a timer that was
// since superseded (unit re-touched, or already fired/removed).
func (w *Watcher) scheduleTimer(unit string, gen uint64, d time.Duration) *time.Timer {
	w.deliverWG.Add(1)
	return time.AfterFunc(d, func() {
		defer w.deliverWG.Done()
		select {
		case w.timerFired <- timerSignal{unit: unit, gen: gen}:
		case <-w.done:
		}
	})
}

// stopTimer stops p's pending timer, if any. Stop returning true means it
// successfully prevented the timer's AfterFunc callback from ever running
// — in which case that callback's matching deliverWG.Done() will never
// happen, so stopTimer accounts for it here instead. Stop returning false
// means the callback already fired (or is running) and will call Done()
// itself; nothing further to do.
func (w *Watcher) stopTimer(p *pendingUnit) {
	if p.timer == nil {
		return
	}
	if p.timer.Stop() {
		w.deliverWG.Done()
	}
	p.timer = nil
}

// deliver sends ev on Events() from its own goroutine so a slow consumer
// blocks only this event's delivery, never run()'s processing of other
// units or the next raw event.
func (w *Watcher) deliver(ev Event) {
	w.deliverWG.Add(1)
	go func() {
		defer w.deliverWG.Done()
		select {
		case w.events <- ev:
		case <-w.done:
		}
	}()
}

func (w *Watcher) deliverError(ctx context.Context, err error) {
	w.logger.WarnContext(ctx, "fswatch error", "error", err)
	w.deliverWG.Add(1)
	go func() {
		defer w.deliverWG.Done()
		select {
		case w.errors <- err:
		case <-w.done:
		}
	}()
}

func matchesAny(globs []string, name string) bool {
	for _, g := range globs {
		if ok, _ := filepath.Match(g, name); ok {
			return true
		}
	}
	return false
}
