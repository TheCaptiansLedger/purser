// Package badger implements the datastore.Datastore port against a
// BadgerDB v4 backend. See docs/adr/0012-datastore-persistence.md.
package badger

import (
	"errors"
	"fmt"

	badgerdb "github.com/dgraph-io/badger/v4"
)

const currentSchemaVersion = "1"

// Options configures Open. Purser's composition root (cmd/purser) is the
// only place responsible for mapping internal/config values into this —
// this package never imports internal/config, keeping the adapter layer
// decoupled from configuration per docs/adr/0001-hexagonal-architecture.md.
type Options struct {
	// DataDir is the directory Badger stores its LSM-tree files in.
	DataDir string
	// ValueLogDir optionally places the value log on a separate disk.
	// Empty means "same as DataDir".
	ValueLogDir string
	// SyncWrites flushes every write to disk before returning. Off by
	// default for throughput; on trades throughput for durability.
	SyncWrites bool
}

// Open opens (or creates) a BadgerDB database at opts.DataDir, then checks
// and stamps the schema version.
func Open(opts Options) (*badgerdb.DB, error) {
	bopts := badgerdb.DefaultOptions(opts.DataDir)
	if opts.ValueLogDir != "" {
		bopts = bopts.WithValueDir(opts.ValueLogDir)
	}
	bopts = bopts.WithSyncWrites(opts.SyncWrites)
	// 64 MiB cap prevents the default 1 GiB pre-allocation from inflating
	// on-disk size reports for small databases.
	bopts = bopts.WithValueLogFileSize(64 << 20)
	// Silence BadgerDB's internal logger; Purser uses slog.
	bopts = bopts.WithLogger(nil)

	db, err := badgerdb.Open(bopts)
	if err != nil {
		return nil, fmt.Errorf("datastore/badger: open: %w", err)
	}

	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("datastore/badger: schema: %w", err)
	}

	return db, nil
}

// ensureSchema checks the stored schema version and applies migrations as
// needed. There is exactly one document/index keyspace shape (see
// docs/adr/0012-datastore-persistence.md), so there is nothing to migrate
// yet — this exists so a future shape change has somewhere to hook in.
func ensureSchema(db *badgerdb.DB) error {
	return db.Update(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(kSchema())
		if errors.Is(err, badgerdb.ErrKeyNotFound) {
			return txn.Set(kSchema(), []byte(currentSchemaVersion))
		}
		if err != nil {
			return fmt.Errorf("read schema version: %w", err)
		}
		var stored string
		if err := item.Value(func(val []byte) error {
			stored = string(val)
			return nil
		}); err != nil {
			return fmt.Errorf("read schema version value: %w", err)
		}
		if stored == currentSchemaVersion {
			return nil
		}
		// Future: apply versioned migrations here.
		return txn.Set(kSchema(), []byte(currentSchemaVersion))
	})
}
