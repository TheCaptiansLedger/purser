package service

import (
	"context"
	"log/slog"
	"purser/internal/ports"
	"purser/pkg/fswatch"
)

// ScanWatchConsumer drives ScanService.Trigger from a live
// ports.FileWatcher's settled events — the automatic half of
// docs/adr/0024-pipeline-core.md's "Discovery: both a watcher and an
// on-demand recursive scan, one code path." It calls the identical
// Trigger method the TriggerScan RPC calls explicitly; this is not a
// second scan implementation, just a second trigger for the same one.
type ScanWatchConsumer struct {
	watcher ports.FileWatcher
	scanSvc *ScanService
	logger  *slog.Logger
}

// NewScanWatchConsumer constructs a ScanWatchConsumer backed by watcher
// and scanSvc. logger defaults to slog.Default() if nil, per
// docs/adr/0007-telemetry.md's no-cost-to-opt-out convention.
func NewScanWatchConsumer(watcher ports.FileWatcher, scanSvc *ScanService, logger *slog.Logger) *ScanWatchConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanWatchConsumer{
		watcher: watcher,
		scanSvc: scanSvc,
		logger:  logger.With("component", "service.scan_watch_consumer"),
	}
}

// Run reads events and errors from the watcher until ctx is done or both
// channels close. Each settled Created/Updated event triggers exactly one
// scan, scoped to the event's unit path rather than the whole configured
// root — one TriggerScan call per debounced event, not one per raw
// filesystem event. Removed events need no scan (nothing to hash) and are
// skipped. Watcher errors are logged via slog, never silently dropped,
// and never stop the loop.
func (c *ScanWatchConsumer) Run(ctx context.Context) {
	events := c.watcher.Events()
	errs := c.watcher.Errors()

	for events != nil || errs != nil {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			c.handleEvent(ctx, ev)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			c.logger.ErrorContext(ctx, "filesystem watch error", "error", err)
		}
	}
}

func (c *ScanWatchConsumer) handleEvent(ctx context.Context, ev fswatch.Event) {
	if ev.Kind == fswatch.Removed {
		return
	}

	jobID, err := c.scanSvc.Trigger(ctx, ev.Path)
	if err != nil {
		c.logger.ErrorContext(ctx, "triggering scan from watch event",
			"error", err, "root", ev.Root, "unit", ev.Path, "kind", ev.Kind.String())
		return
	}
	c.logger.InfoContext(ctx, "triggered scan from watch event",
		"job.id", jobID, "root", ev.Root, "unit", ev.Path, "kind", ev.Kind.String())
}
