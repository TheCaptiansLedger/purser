package ports

import (
	"context"
	"io"
)

// DatabaseInfo is the read-only snapshot GetDatabaseInfo (see
// docs/adr/0011-api-design.md) surfaces about the active Datastore
// backend: which driver is running, its version, how much storage it's
// using, and how many documents live in each collection.
type DatabaseInfo struct {
	// Driver names the active backend: "badger", "postgres", "mysql", or
	// "sqlite" — mirrors internal/config.Database.Driver.
	Driver string
	// Version is the backend's own version string (the pinned Badger
	// module version, or the SQL engine's reported server version).
	// "unknown" if it can't be determined.
	Version string
	// StorageSizeBytes is the backend's total on-disk footprint.
	StorageSizeBytes int64
	// CollectionCounts maps each collection name to its document count.
	CollectionCounts map[string]int64
}

// DatabaseAdmin is the narrow seam DatabaseService depends on for
// database-wide administration — info, backup, restore — distinct from
// and never widening Datastore, which stays reserved for entity CRUD (see
// docs/adr/0012-datastore-persistence.md). See
// docs/technical/database-backup-restore.md for the backup artifact
// format this interface's Backup/Restore agree on, and
// docs/adr/0002-solid-design-principles.md's Interface Segregation note
// for why this is its own port rather than added to Datastore.
//
// A DatabaseAdmin implementation holds both the backend's raw handle (for
// the Backup/Restore keyspace/table walk, which by design bypasses the
// Datastore/Document abstraction entirely) and a reference to the
// corresponding datastore.Datastore (for Restore's CreateBatch replay).
type DatabaseAdmin interface {
	// Info returns the current DatabaseInfo snapshot.
	Info(ctx context.Context) (DatabaseInfo, error)

	// Backup writes the full backup artifact (a version-stamped JSONL
	// stream of every document in the database, see
	// docs/technical/database-backup-restore.md) to w, in one
	// consistent, point-in-time pass.
	Backup(ctx context.Context, w io.Writer) error

	// Restore validates the backup artifact read from r, then destroys
	// and replaces every document currently in the database with the
	// artifact's contents. r is expected to be re-readable from the
	// start (an *os.File in practice — the caller stages the upload to
	// disk before calling Restore) so the implementation can fully
	// validate the stream before mutating anything. A non-nil error
	// means the database was left untouched; restart of the process is
	// the caller's responsibility once Restore returns successfully.
	Restore(ctx context.Context, r io.Reader) error
}
