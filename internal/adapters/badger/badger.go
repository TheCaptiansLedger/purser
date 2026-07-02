// Package badger implements all repository ports against a BadgerDB v4 backend.
// BadgerDB is a pure-Go embedded LSM-tree key-value store with MVCC transactions
// and a native streaming backup/restore API.
package badger

import (
	"errors"
	"fmt"
	"purser/internal/config"

	"github.com/dgraph-io/badger/v4"
)

const currentSchemaVersion = "1"

// Open opens (or creates) a BadgerDB database using the supplied configuration,
// then checks and stamps the schema version.
func Open(cfg config.BadgerConfig) (*badger.DB, error) {
	opts := badger.DefaultOptions(cfg.DataDir)
	if cfg.ValueLogDir != "" {
		opts = opts.WithValueDir(cfg.ValueLogDir)
	}
	opts = opts.WithSyncWrites(cfg.SyncWrites)
	// Silence BadgerDB's internal logger; Purser uses slog.
	opts = opts.WithLogger(nil)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("badger schema: %w", err)
	}

	return db, nil
}

// ensureSchema checks the stored schema version and applies migrations as needed.
func ensureSchema(db *badger.DB) error {
	return db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(kSchema())
		if errors.Is(err, badger.ErrKeyNotFound) {
			return txn.Set(kSchema(), []byte(currentSchemaVersion))
		}
		if err != nil {
			return fmt.Errorf("read schema version: %w", err)
		}
		var stored string
		_ = item.Value(func(val []byte) error {
			stored = string(val)
			return nil
		})
		if stored == currentSchemaVersion {
			return nil
		}
		// Future: apply versioned migrations here.
		return txn.Set(kSchema(), []byte(currentSchemaVersion))
	})
}
