// Package sql implements ports.DatabaseAdmin against the SQL backend
// (PostgreSQL, MySQL, SQLite). See docs/technical/database-backup-restore.md
// and docs/adr/0012-datastore-persistence.md.
package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"purser/internal/adapters/database"
	"purser/internal/adapters/datastore"
	"purser/internal/ports"

	sqlstore "purser/internal/adapters/datastore/sql"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/internal/adapters/database/sql"

// Admin implements ports.DatabaseAdmin against a SQL backend, holding
// both the raw *sql.DB handle (for the table walk Backup/Restore need —
// see docs/technical/database-backup-restore.md's §2.3/§3, which reads
// documents/document_index directly rather than going through Datastore)
// and the corresponding datastore.Datastore (for Restore's CreateBatch
// replay).
//
// Unlike internal/adapters/database/badger, no exported helpers are
// needed from internal/adapters/datastore/sql: documents/document_index
// and their columns are already the public, documented schema
// docs/adr/0012-datastore-persistence.md commits to, not a private
// on-disk encoding — so Admin issues its own plain SQL directly. None of
// its statements take placeholders, so datastore/sql's unexported rebind
// helper isn't needed either.
type Admin struct {
	db      *sql.DB
	dialect sqlstore.Dialect
	ds      datastore.Datastore

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

// New constructs an Admin backed by db (the same *sql.DB handle
// datastore/sql.Open returned) for dialect and ds (the Datastore built on
// top of it).
func New(db *sql.DB, dialect sqlstore.Dialect, ds datastore.Datastore, opts ...Option) (*Admin, error) {
	if db == nil {
		return nil, fmt.Errorf("adapters/database/sql: db must not be nil")
	}
	if ds == nil {
		return nil, fmt.Errorf("adapters/database/sql: ds must not be nil")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Admin{
		db:      db,
		dialect: dialect,
		ds:      ds,
		logger:  o.logger.With("component", "adapters.database.sql"),
		tracer:  o.tracerProvider.Tracer(instrumentationName),
	}, nil
}

// Info implements ports.DatabaseAdmin.
func (a *Admin) Info(ctx context.Context) (ports.DatabaseInfo, error) {
	ctx, span := a.tracer.Start(ctx, "database_sql.info")
	defer span.End()

	version, err := a.version(ctx)
	if err != nil {
		return ports.DatabaseInfo{}, fmt.Errorf("adapters/database/sql: info: %w", err)
	}
	size, err := a.storageSize(ctx)
	if err != nil {
		return ports.DatabaseInfo{}, fmt.Errorf("adapters/database/sql: info: %w", err)
	}
	counts, err := a.collectionCounts(ctx)
	if err != nil {
		return ports.DatabaseInfo{}, fmt.Errorf("adapters/database/sql: info: %w", err)
	}

	a.logger.DebugContext(ctx, "database info", "collection.count", len(counts))
	return ports.DatabaseInfo{
		Driver:           string(a.dialect),
		Version:          version,
		StorageSizeBytes: size,
		CollectionCounts: counts,
	}, nil
}

// version reports the SQL engine's own reported server version — the
// only genuinely dialect-specific query in this file, same "written once
// except where the dialects truly diverge" spirit as
// docs/adr/0012-datastore-persistence.md's rebind/isConflict.
func (a *Admin) version(ctx context.Context) (string, error) {
	var query string
	switch a.dialect {
	case sqlstore.DialectPostgres:
		query = "SELECT version()"
	case sqlstore.DialectMySQL:
		query = "SELECT VERSION()"
	case sqlstore.DialectSQLite:
		query = "SELECT sqlite_version()"
	default:
		return "unknown", nil
	}
	var v string
	if err := a.db.QueryRowContext(ctx, query).Scan(&v); err != nil {
		return "", fmt.Errorf("query version: %w", err)
	}
	return v, nil
}

// storageSize reports the database's total on-disk footprint via each
// dialect's own accounting mechanism — there is no portable ANSI-SQL way
// to ask this.
func (a *Admin) storageSize(ctx context.Context) (int64, error) {
	switch a.dialect {
	case sqlstore.DialectPostgres:
		var size int64
		if err := a.db.QueryRowContext(ctx, "SELECT pg_database_size(current_database())").Scan(&size); err != nil {
			return 0, fmt.Errorf("query storage size: %w", err)
		}
		return size, nil
	case sqlstore.DialectMySQL:
		var size int64
		if err := a.db.QueryRowContext(ctx,
			"SELECT COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables WHERE table_schema = DATABASE()",
		).Scan(&size); err != nil {
			return 0, fmt.Errorf("query storage size: %w", err)
		}
		return size, nil
	case sqlstore.DialectSQLite:
		var pageCount, pageSize int64
		if err := a.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
			return 0, fmt.Errorf("query page_count: %w", err)
		}
		if err := a.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
			return 0, fmt.Errorf("query page_size: %w", err)
		}
		return pageCount * pageSize, nil
	default:
		return 0, fmt.Errorf("unknown dialect %q", a.dialect)
	}
}

// collectionCounts computes per-collection document counts directly —
// documents already carries a collection column, so this is a single
// indexed GROUP BY, no walk needed (contrast with the Badger adapter,
// which has to derive collection from the key itself).
func (a *Admin) collectionCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT collection, COUNT(*) FROM documents GROUP BY collection")
	if err != nil {
		return nil, fmt.Errorf("query collection counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int64)
	for rows.Next() {
		var collection string
		var count int64
		if err := rows.Scan(&collection, &count); err != nil {
			return nil, fmt.Errorf("scan collection count: %w", err)
		}
		counts[collection] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collection counts: %w", err)
	}
	return counts, nil
}

// Backup implements ports.DatabaseAdmin: a single ordered, streamed query
// over documents — design doc §2.3 verbatim. document_index and
// schema_migrations are deliberately not read: document_index is fully
// reconstructible from index_json on restore via the normal write path,
// and schema_migrations is process/backend state, not library data.
func (a *Admin) Backup(ctx context.Context, w io.Writer) error {
	ctx, span := a.tracer.Start(ctx, "database_sql.backup")
	defer span.End()

	if err := database.WriteHeader(w); err != nil {
		return fmt.Errorf("adapters/database/sql: backup: %w", err)
	}

	rows, err := a.db.QueryContext(ctx, "SELECT collection, id, data, index_json FROM documents ORDER BY collection, id")
	if err != nil {
		return fmt.Errorf("adapters/database/sql: backup: %w", err)
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var collection, id, data, indexJSON string
		if err := rows.Scan(&collection, &id, &data, &indexJSON); err != nil {
			return fmt.Errorf("adapters/database/sql: backup: scan row: %w", err)
		}
		var index map[string]string
		if indexJSON != "" && indexJSON != "{}" {
			if err := json.Unmarshal([]byte(indexJSON), &index); err != nil {
				return fmt.Errorf("adapters/database/sql: backup: decode index_json for %s/%s: %w", collection, id, err)
			}
		}
		if err := database.WriteDocument(w, datastore.Document{Collection: collection, ID: id, Data: []byte(data), Index: index}); err != nil {
			return fmt.Errorf("adapters/database/sql: backup: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("adapters/database/sql: backup: %w", err)
	}

	a.logger.InfoContext(ctx, "database backup complete", "document.count", count)
	return nil
}

// Restore implements ports.DatabaseAdmin, delegating to
// database.ApplyRestore with a's own destructive clear step.
func (a *Admin) Restore(ctx context.Context, r io.Reader) error {
	ctx, span := a.tracer.Start(ctx, "database_sql.restore")
	defer span.End()

	if err := database.ApplyRestore(ctx, r, a.ds, a.clear); err != nil {
		return fmt.Errorf("adapters/database/sql: restore: %w", err)
	}

	a.logger.InfoContext(ctx, "database restore complete")
	return nil
}

// clear deletes every row from document_index then documents — plain
// DELETE, not TRUNCATE (SQLite has no TRUNCATE), per the design doc's §3.
// schema_migrations is left untouched: restore assumes it's running
// against a datastore that's already been through Open()'s migrations.
func (a *Admin) clear(ctx context.Context) error {
	if _, err := a.db.ExecContext(ctx, "DELETE FROM document_index"); err != nil {
		return fmt.Errorf("clearing document_index: %w", err)
	}
	if _, err := a.db.ExecContext(ctx, "DELETE FROM documents"); err != nil {
		return fmt.Errorf("clearing documents: %w", err)
	}
	return nil
}
