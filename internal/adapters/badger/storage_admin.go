package badger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"purser/internal/ports"
	"runtime/debug"

	badgerdb "github.com/dgraph-io/badger/v4"
)

// StorageAdmin implements ports.StorageAdminPort for the BadgerDB backend.
type StorageAdmin struct {
	db      *badgerdb.DB
	dataDir string
}

// NewStorageAdmin returns a StorageAdmin backed by the given open database and data directory.
func NewStorageAdmin(db *badgerdb.DB, dataDir string) ports.StorageAdminPort {
	return &StorageAdmin{db: db, dataDir: dataDir}
}

var _ ports.StorageAdminPort = (*StorageAdmin)(nil)

// DriverName returns "badger".
func (a *StorageAdmin) DriverName() string { return "badger" }

// BackupMeta returns the content-type and suggested filename for a BadgerDB backup.
// The backup format is a binary protobuf stream (not SQL).
func (a *StorageAdmin) BackupMeta() (string, string) {
	return "application/octet-stream", "purser.badger"
}

// Stats counts primary records by key prefix, measures disk usage, and reports LSM/vlog sizes.
func (a *StorageAdmin) Stats(_ context.Context) (*ports.StorageStats, error) {
	type prefixDef struct {
		name   string
		prefix []byte
	}
	prefixes := []prefixDef{
		{"library_entries", pfxLE()},
		{"groups", pfxGRP()},
		{"items", pfxITM()},
		{"people", pfxPER()},
		{"tags", pfxTAG()},
		{"media_files", []byte("mf:")},
		{"settings", []byte("cfg:")},
	}

	collections := make([]ports.CollectionStats, len(prefixes))
	if err := a.db.View(func(txn *badgerdb.Txn) error {
		for i, p := range prefixes {
			var count int64
			iterPrefix(txn, p.prefix, func(_ []byte) bool {
				count++
				return true
			})
			collections[i] = ports.CollectionStats{Name: p.name, Count: count}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("badger stats: %w", err)
	}

	var sizeBytes int64
	_ = filepath.WalkDir(a.dataDir, func(_ string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // best-effort; skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			sizeBytes += info.Size()
		}
		return nil
	})

	lsmSize, vlogSize := a.db.Size()

	return &ports.StorageStats{
		Driver:        "badger",
		DriverVersion: badgerModuleVersion(),
		SizeBytes:     sizeBytes,
		Collections:   collections,
		Extra: map[string]any{
			"lsm_size_bytes":  lsmSize,
			"vlog_size_bytes": vlogSize,
		},
	}, nil
}

// Backup streams the entire database using BadgerDB's native protobuf backup format.
func (a *StorageAdmin) Backup(_ context.Context, w io.Writer) error {
	_, err := a.db.Backup(w, 0)
	return err
}

// Restore loads a BadgerDB backup, validates it, atomically replaces the live database
// directory, and triggers shutdownFn so the process restarts with the restored data.
func (a *StorageAdmin) Restore(_ context.Context, r io.Reader, shutdownFn func()) (*ports.StorageStats, error) {
	// Detect a SQLite SQL dump (starts with "--") so we can return a typed error
	// instead of a confusing protobuf parse failure.
	peek := make([]byte, 2)
	n, err := io.ReadFull(r, peek)
	if n == 0 && err != nil {
		return nil, fmt.Errorf("read backup header: %w", err)
	}
	if bytes.HasPrefix(peek[:n], []byte("--")) {
		return nil, fmt.Errorf("INVALID_FORMAT: uploaded file is a SQL dump, not a BadgerDB backup")
	}
	r = io.MultiReader(bytes.NewReader(peek[:n]), r)

	tmpDir, err := os.MkdirTemp(filepath.Dir(a.dataDir), "purser-restore-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	tmpOpts := badgerdb.DefaultOptions(tmpDir).WithLogger(nil)
	tmpDB, err := badgerdb.Open(tmpOpts)
	if err != nil {
		return nil, fmt.Errorf("open restore staging db: %w", err)
	}

	if err := tmpDB.Load(r, 256); err != nil {
		_ = tmpDB.Close()
		return nil, fmt.Errorf("load backup data: %w", err)
	}

	// Validate that the backup came from Purser by checking the schema key.
	if err := tmpDB.View(func(txn *badgerdb.Txn) error {
		_, err := txn.Get(kSchema())
		return err
	}); err != nil {
		_ = tmpDB.Close()
		return nil, fmt.Errorf("not a valid Purser database (schema key missing): %w", err)
	}

	stats, err := collectStorageStats(tmpDB)
	_ = tmpDB.Close()
	if err != nil {
		return nil, fmt.Errorf("collect restored stats: %w", err)
	}

	// Close the live database before touching the directory on disk.
	if err := a.db.Close(); err != nil {
		return nil, fmt.Errorf("close live database: %w", err)
	}

	bakDir := a.dataDir + ".bak"
	if err := os.Rename(a.dataDir, bakDir); err != nil {
		// DB is closed but files are still in place; force a restart so the
		// caller brings it back up with the original data.
		go shutdownFn()
		return nil, fmt.Errorf("move current database aside: %w", err)
	}

	if err := os.Rename(tmpDir, a.dataDir); err != nil {
		// Try to roll back; if this also fails the operator must intervene.
		_ = os.Rename(bakDir, a.dataDir)
		go shutdownFn()
		cleanup = false
		return nil, fmt.Errorf("replace database: %w", err)
	}
	cleanup = false
	_ = os.RemoveAll(bakDir)

	go shutdownFn()
	return stats, nil
}

// collectStorageStats counts primary records across all key prefixes in db.
func collectStorageStats(db *badgerdb.DB) (*ports.StorageStats, error) {
	type prefixDef struct {
		name   string
		prefix []byte
	}
	prefixes := []prefixDef{
		{"library_entries", pfxLE()},
		{"groups", pfxGRP()},
		{"items", pfxITM()},
		{"people", pfxPER()},
		{"tags", pfxTAG()},
		{"media_files", []byte("mf:")},
		{"settings", []byte("cfg:")},
	}

	collections := make([]ports.CollectionStats, len(prefixes))
	if err := db.View(func(txn *badgerdb.Txn) error {
		for i, p := range prefixes {
			var count int64
			iterPrefix(txn, p.prefix, func(_ []byte) bool {
				count++
				return true
			})
			collections[i] = ports.CollectionStats{Name: p.name, Count: count}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &ports.StorageStats{Collections: collections}, nil
}

// badgerModuleVersion returns the version string of the BadgerDB module as reported
// by the Go runtime build info. Falls back to "unknown" if not available.
func badgerModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/dgraph-io/badger/v4" {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "unknown"
}
