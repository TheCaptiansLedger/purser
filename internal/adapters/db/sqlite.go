// Package db implements the SQLite storage adapter. All SQLite-specific
// configuration (driver registration, PRAGMAs, single-connection pool) lives
// here. A future adapters/db/postgres.go would provide the same Open signature
// wired to a Postgres driver instead.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open opens (or creates) a SQLite database at dsn, applies WAL mode and
// foreign key enforcement, and runs any pending schema migrations.
//
// SetMaxOpenConns(1) is intentional: SQLite PRAGMAs are per-connection, so
// restricting the pool to one connection ensures the PRAGMA settings applied
// below hold for every query. All repository implementations avoid holding a
// cursor open while issuing secondary queries (batch-load pattern) so the
// single connection is never a bottleneck.
func Open(dsn string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	database.SetMaxOpenConns(1)

	if _, err := database.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}

	if err := runMigrations(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return database, nil
}

func runMigrations(database *sql.DB) error {
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	rows, err := database.Query(`SELECT version FROM schema_migrations ORDER BY version`)
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
		if err := applyMigration(database, f, data); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(database *sql.DB, filename string, data []byte) error {
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", filename, err)
	}

	if _, err := tx.Exec(string(data)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", filename, err)
	}

	if _, err := tx.Exec(
		`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`,
		filename, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", filename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", filename, err)
	}
	return nil
}
