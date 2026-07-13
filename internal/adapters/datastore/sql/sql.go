// Package sql implements the datastore.Datastore port against
// PostgreSQL, MySQL, and SQLite via database/sql. See
// docs/adr/0012-datastore-persistence.md.
package sql

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers driver "mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers driver "pgx"
	_ "modernc.org/sqlite"             // registers driver "sqlite", pure Go, no CGo
)

// migrationFiles holds the one shared migration tree used by every
// dialect — see docs/adr/0012-datastore-persistence.md. Each file must
// contain exactly one DDL statement: some drivers (notably pgx's default
// extended protocol) reject a query string containing more than one
// semicolon-separated command.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Options configures Open. Purser's composition root (cmd/purser) is the
// only place responsible for mapping internal/config values into this —
// this package never imports internal/config, keeping the adapter layer
// decoupled from configuration per docs/adr/0001-hexagonal-architecture.md.
type Options struct {
	Dialect Dialect
	DSN     string
}

// Open opens (or creates) a database for opts.Dialect/opts.DSN and runs
// any pending schema migrations.
func Open(opts Options) (*sql.DB, error) {
	driver, err := driverName(opts.Dialect)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driver, opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("datastore/sql: open: %w", err)
	}

	if opts.Dialect == DialectSQLite {
		// SQLite PRAGMAs are per-connection; a single-connection pool
		// ensures WAL/foreign_keys hold for every query issued through
		// db. Store never holds a cursor open while issuing a secondary
		// query, so a single connection is never a bottleneck.
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("datastore/sql: configure sqlite: %w", err)
		}
	}

	if err := runMigrations(db, opts.Dialect); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("datastore/sql: migrate: %w", err)
	}

	return db, nil
}

func driverName(d Dialect) (string, error) {
	switch d {
	case DialectPostgres:
		return "pgx", nil
	case DialectMySQL:
		return "mysql", nil
	case DialectSQLite:
		return "sqlite", nil
	default:
		return "", fmt.Errorf("datastore/sql: unknown dialect %q", d)
	}
}

func runMigrations(db *sql.DB, dialect Dialect) error {
	schemaTable := rebind(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(255) PRIMARY KEY,
		applied_at VARCHAR(64) NOT NULL
	)`, dialect)
	if _, err := db.Exec(schemaTable); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	rows, err := db.Query(rebind(`SELECT version FROM schema_migrations ORDER BY version`, dialect))
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return err
		}
		applied[v] = true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migration dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		if applied[f] {
			continue
		}
		data, err := migrationFiles.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if err := applyMigration(db, dialect, f, string(data)); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(db *sql.DB, dialect Dialect, filename, ddl string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", filename, err)
	}

	if _, err := tx.Exec(ddl); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", filename, err)
	}

	insert := rebind(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`, dialect)
	if _, err := tx.Exec(insert, filename, time.Now().UTC().Format(time.RFC3339)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", filename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", filename, err)
	}
	return nil
}
