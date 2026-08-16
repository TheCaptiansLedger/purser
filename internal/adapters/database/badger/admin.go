// Package badger implements ports.DatabaseAdmin against a Badger backend.
// See docs/technical/database-backup-restore.md and
// docs/adr/0012-datastore-persistence.md.
package badger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"purser/internal/adapters/database"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"
	"runtime/debug"

	dsbadger "purser/internal/adapters/datastore/badger"

	badgerdb "github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName = "purser/internal/adapters/database/badger"
	badgerModulePath    = "github.com/dgraph-io/badger/v4"
	// deleteBatchSize bounds how many keys Restore's clear step deletes
	// per transaction — same shape as the datastore/badger package's own
	// batch operations.
	deleteBatchSize = 500
	// clearTxnRetries bounds how many times a delete transaction retries
	// on Badger's own transient badgerdb.ErrConflict, mirroring
	// datastore/badger's updateTxnRetries.
	clearTxnRetries = 5
)

// Admin implements ports.DatabaseAdmin against a Badger backend, holding
// both the raw *badger.DB handle (for the keyspace walk Backup/Restore
// need — see docs/technical/database-backup-restore.md, which is
// deliberately a raw walk bypassing Datastore/Document) and the
// corresponding datastore.Datastore (for Restore's CreateBatch replay).
type Admin struct {
	db *badgerdb.DB
	ds datastore.Datastore

	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.DatabaseAdmin = (*Admin)(nil)

// Option configures New.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
}

func defaultOptions() *options {
	return &options{logger: slog.Default(), tracerProvider: otel.GetTracerProvider()}
}

// WithLogger overrides the default (slog.Default()) logger.
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.logger = l } }

// WithTracerProvider overrides the default (otel.GetTracerProvider())
// tracer provider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) { o.tracerProvider = tp }
}

// New constructs an Admin backed by db (the same *badger.DB handle
// datastore/badger.Open returned) and ds (the Datastore built on top of
// it).
func New(db *badgerdb.DB, ds datastore.Datastore, opts ...Option) (*Admin, error) {
	if db == nil {
		return nil, fmt.Errorf("adapters/database/badger: db must not be nil")
	}
	if ds == nil {
		return nil, fmt.Errorf("adapters/database/badger: ds must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Admin{
		db:     db,
		ds:     ds,
		logger: o.logger.With("component", "adapters.database.badger"),
		tracer: o.tracerProvider.Tracer(instrumentationName),
	}, nil
}

// Info implements ports.DatabaseAdmin. Per-collection counts come from
// the same DocumentPrefix() walk Backup uses, computed here as a KeyOnly
// pass (no value decoding needed just to count) — see the design doc's
// §4 note that this is a free side effect of the walk shape, not a
// separate pass.
func (a *Admin) Info(ctx context.Context) (ports.DatabaseInfo, error) {
	ctx, span := a.tracer.Start(ctx, "database_badger.info")
	defer span.End()

	lsm, vlog := a.db.Size()

	counts := make(map[string]int64)
	prefix := dsbadger.DocumentPrefix()
	err := a.db.View(func(txn *badgerdb.Txn) error {
		iterOpts := badgerdb.DefaultIteratorOptions
		iterOpts.PrefetchValues = false
		it := txn.NewIterator(iterOpts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			collection, _, ok := dsbadger.SplitDocumentKey(it.Item().KeyCopy(nil))
			if !ok {
				continue
			}
			counts[collection]++
		}
		return nil
	})
	if err != nil {
		return ports.DatabaseInfo{}, fmt.Errorf("adapters/database/badger: info: %w", err)
	}

	a.logger.DebugContext(ctx, "database info", "collection.count", len(counts))
	return ports.DatabaseInfo{
		Driver:           "badger",
		Version:          badgerModuleVersion(),
		StorageSizeBytes: lsm + vlog,
		CollectionCounts: counts,
	}, nil
}

// badgerModuleVersion reports the pinned github.com/dgraph-io/badger/v4
// module version this binary was built with, via the build info Go
// embeds automatically — "unknown" if that information isn't available
// (e.g. a binary built with `go build` outside module mode).
func badgerModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == badgerModulePath {
			return dep.Version
		}
	}
	return "unknown"
}

// Backup implements ports.DatabaseAdmin: one db.View transaction —
// Badger's own MVCC point-in-time snapshot, safe to take while the
// process keeps serving writes — prefix-scanning DocumentPrefix() yields
// every document across every collection in one consistent pass. See the
// design doc's §2.2.
func (a *Admin) Backup(ctx context.Context, w io.Writer) error {
	ctx, span := a.tracer.Start(ctx, "database_badger.backup")
	defer span.End()

	if err := database.WriteHeader(w); err != nil {
		return fmt.Errorf("adapters/database/badger: backup: %w", err)
	}

	prefix := dsbadger.DocumentPrefix()
	count := 0
	err := a.db.View(func(txn *badgerdb.Txn) error {
		it := txn.NewIterator(badgerdb.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			collection, id, ok := dsbadger.SplitDocumentKey(item.KeyCopy(nil))
			if !ok {
				continue
			}
			if err := item.Value(func(val []byte) error {
				data, index, err := dsbadger.DecodeEnvelope(val)
				if err != nil {
					return err
				}
				return database.WriteDocument(w, datastore.Document{Collection: collection, ID: id, Data: []byte(data), Index: index})
			}); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("adapters/database/badger: backup: %w", err)
	}

	a.logger.InfoContext(ctx, "database backup complete", "document.count", count)
	return nil
}

// Restore implements ports.DatabaseAdmin, delegating to
// database.ApplyRestore with a's own destructive clear step.
func (a *Admin) Restore(ctx context.Context, r io.Reader) error {
	ctx, span := a.tracer.Start(ctx, "database_badger.restore")
	defer span.End()

	if err := database.ApplyRestore(ctx, r, a.ds, a.clear); err != nil {
		return fmt.Errorf("adapters/database/badger: restore: %w", err)
	}

	a.logger.InfoContext(ctx, "database restore complete")
	return nil
}

// clear deletes every document and secondary-index key — Restore's
// destructive first step, per the design doc's §3.
func (a *Admin) clear(context.Context) error {
	if err := a.deletePrefix(dsbadger.DocumentPrefix()); err != nil {
		return err
	}
	return a.deletePrefix(dsbadger.IndexPrefix())
}

// deletePrefix deletes every key under prefix in bounded, retried
// batches: keys are collected in one read transaction, then deleted in a
// separate write transaction (Badger iterators are a point-in-time
// snapshot, not a live cursor a concurrent Delete can safely advance),
// repeating until no keys remain under prefix.
func (a *Admin) deletePrefix(prefix []byte) error {
	for {
		var keys [][]byte
		if err := a.db.View(func(txn *badgerdb.Txn) error {
			iterOpts := badgerdb.DefaultIteratorOptions
			iterOpts.PrefetchValues = false
			it := txn.NewIterator(iterOpts)
			defer it.Close()
			for it.Seek(prefix); it.ValidForPrefix(prefix) && len(keys) < deleteBatchSize; it.Next() {
				keys = append(keys, it.Item().KeyCopy(nil))
			}
			return nil
		}); err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}

		var lastErr error
		for attempt := 0; attempt < clearTxnRetries; attempt++ {
			lastErr = a.db.Update(func(txn *badgerdb.Txn) error {
				for _, k := range keys {
					if err := txn.Delete(k); err != nil {
						return err
					}
				}
				return nil
			})
			if lastErr == nil || !errors.Is(lastErr, badgerdb.ErrConflict) {
				break
			}
		}
		if lastErr != nil {
			return lastErr
		}
	}
}
