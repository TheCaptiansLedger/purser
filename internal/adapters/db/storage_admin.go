package db

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"purser/internal/ports"
	"strings"
	"time"
)

// StorageAdmin implements ports.StorageAdminPort for the SQLite backend.
type StorageAdmin struct {
	db  *sql.DB
	dsn string
}

// NewStorageAdmin returns a StorageAdmin backed by the given open database and DSN.
func NewStorageAdmin(db *sql.DB, dsn string) *StorageAdmin {
	return &StorageAdmin{db: db, dsn: dsn}
}

// DriverName returns "sqlite".
func (a *StorageAdmin) DriverName() string { return "sqlite" }

// BackupMeta returns the content-type and suggested filename for a SQLite backup.
func (a *StorageAdmin) BackupMeta() (string, string) {
	return "text/plain; charset=utf-8", "purser.sql"
}

// Stats returns current size, version, table row counts, and migration count.
func (a *StorageAdmin) Stats(ctx context.Context) (*ports.StorageStats, error) {
	var version string
	_ = a.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)

	rows, err := a.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	collections, _ := scanCollections(ctx, rows, a.db)

	var sizeBytes int64
	if info, err := os.Stat(a.dsn); err == nil {
		sizeBytes = info.Size()
	}

	var migCount int
	_ = a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migCount)

	var walSize int64
	if info, err := os.Stat(a.dsn + "-wal"); err == nil {
		walSize = info.Size()
	}

	return &ports.StorageStats{
		Driver:        "sqlite",
		DriverVersion: version,
		SizeBytes:     sizeBytes,
		Collections:   collections,
		Extra: map[string]any{
			"migration_count": migCount,
			"wal_size_bytes":  walSize,
		},
	}, nil
}

// Backup streams a SQL dump of the entire database to w.
func (a *StorageAdmin) Backup(ctx context.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush() //nolint:errcheck

	_, _ = fmt.Fprintf(bw, "-- Purser database dump\n-- Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	_, _ = fmt.Fprintln(bw, "PRAGMA foreign_keys=OFF;")
	_, _ = fmt.Fprintln(bw, "BEGIN TRANSACTION;")

	schemaRows, err := a.db.QueryContext(ctx,
		`SELECT type, name, sql FROM sqlite_master
		 WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'
		 ORDER BY rootpage`)
	if err != nil {
		return fmt.Errorf("query schema: %w", err)
	}

	var tables []string
	for schemaRows.Next() {
		var typ, name, ddl string
		if err := schemaRows.Scan(&typ, &name, &ddl); err != nil {
			continue
		}
		_, _ = fmt.Fprintf(bw, "\n%s;\n", ddl)
		if typ == "table" {
			tables = append(tables, name)
		}
	}
	if err := schemaRows.Err(); err != nil {
		_ = schemaRows.Close()
		return fmt.Errorf("schema rows: %w", err)
	}
	_ = schemaRows.Close()

	for _, table := range tables {
		if err := a.dumpTableData(ctx, bw, table); err != nil {
			return err
		}
	}

	_, _ = fmt.Fprintln(bw, "\nCOMMIT;")
	_, _ = fmt.Fprintln(bw, "PRAGMA foreign_keys=ON;")
	return nil
}

// Restore applies a SQL dump to a fresh database, validates it, atomically
// replaces the live database file, and triggers shutdownFn so the process
// restarts with the restored data.
func (a *StorageAdmin) Restore(ctx context.Context, r io.Reader, shutdownFn func()) (*ports.StorageStats, error) {
	sqlContent, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read restore data: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(a.dsn), "purser-restore-*.db")
	if err != nil {
		return nil, fmt.Errorf("create temp db: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	tmpDB, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("open temp db: %w", err)
	}

	for _, stmt := range splitSQL(string(sqlContent)) {
		if _, err := tmpDB.ExecContext(ctx, stmt); err != nil {
			_ = tmpDB.Close()
			return nil, fmt.Errorf("apply SQL: %w", err)
		}
	}

	var migCount int
	if err := tmpDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'",
	).Scan(&migCount); err != nil || migCount == 0 {
		_ = tmpDB.Close()
		return nil, fmt.Errorf("not a valid Purser database (schema_migrations missing)")
	}

	stats, err := collectAllCollections(ctx, tmpDB)
	_ = tmpDB.Close()
	if err != nil {
		return nil, fmt.Errorf("collect restored stats: %w", err)
	}

	if _, err := a.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return nil, fmt.Errorf("WAL checkpoint: %w", err)
	}

	if err := os.Rename(tmpPath, a.dsn); err != nil {
		return nil, fmt.Errorf("replace database: %w", err)
	}
	cleanup = false

	go shutdownFn()
	return stats, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *StorageAdmin) dumpTableData(ctx context.Context, bw *bufio.Writer, table string) error {
	dataRows, err := a.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %q", table))
	if err != nil {
		return err
	}
	cols, err := dataRows.Columns()
	if err != nil {
		_ = dataRows.Close()
		return err
	}
	for dataRows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := dataRows.Scan(ptrs...); err != nil {
			_ = dataRows.Close()
			return err
		}
		_, _ = fmt.Fprintf(bw, "INSERT INTO %q VALUES (", table)
		for i, v := range vals {
			if i > 0 {
				_, _ = fmt.Fprint(bw, ",")
			}
			writeSQLVal(bw, v)
		}
		_, _ = fmt.Fprintln(bw, ");")
	}
	if err := dataRows.Err(); err != nil {
		_ = dataRows.Close()
		return err
	}
	return dataRows.Close()
}

func scanCollections(ctx context.Context, nameRows *sql.Rows, db *sql.DB) ([]ports.CollectionStats, int64) {
	var names []string
	for nameRows.Next() {
		var name string
		if err := nameRows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	collections := make([]ports.CollectionStats, 0, len(names))
	var total int64
	for _, name := range names {
		var count int64
		_ = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", name)).Scan(&count) //nolint:gosec // name comes from sqlite_master, not user input
		collections = append(collections, ports.CollectionStats{Name: name, Count: count})
		total += count
	}
	return collections, total
}

func collectAllCollections(ctx context.Context, db *sql.DB) (*ports.StorageStats, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	collections, total := scanCollections(ctx, rows, db)
	_ = total
	return &ports.StorageStats{Collections: collections}, nil
}

func writeSQLVal(w io.Writer, v any) {
	switch val := v.(type) {
	case nil:
		_, _ = fmt.Fprint(w, "NULL")
	case int64:
		_, _ = fmt.Fprintf(w, "%d", val)
	case float64:
		_, _ = fmt.Fprintf(w, "%g", val)
	case string:
		_, _ = fmt.Fprintf(w, "'%s'", strings.ReplaceAll(val, "'", "''"))
	case []byte:
		_, _ = fmt.Fprintf(w, "X'%s'", hex.EncodeToString(val))
	default:
		_, _ = fmt.Fprintf(w, "'%v'", val)
	}
}

func splitSQL(input string) []string {
	var stmts []string
	var cur strings.Builder
	inStr := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if inStr {
			i = consumeInString(input, i, &cur, &inStr)
			continue
		}

		switch c {
		case '\'':
			inStr = true
			cur.WriteByte(c)
		case '-':
			if i+1 < len(input) && input[i+1] == '-' {
				i = skipLineComment(input, i)
			} else {
				cur.WriteByte(c)
			}
		case ';':
			cur.WriteByte(c)
			if s := strings.TrimSpace(cur.String()); s != "" && s != ";" {
				stmts = append(stmts, s)
			}
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}

	if s := strings.TrimSpace(cur.String()); s != "" {
		stmts = append(stmts, s)
	}

	return stmts
}

func consumeInString(input string, i int, cur *strings.Builder, inStr *bool) int {
	c := input[i]
	cur.WriteByte(c)
	if c == '\'' {
		if i+1 < len(input) && input[i+1] == '\'' {
			cur.WriteByte(input[i+1])
			return i + 1
		}
		*inStr = false
	}
	return i
}

func skipLineComment(input string, i int) int {
	for i < len(input) && input[i] != '\n' {
		i++
	}
	return i
}

// compile-time interface check
var _ ports.StorageAdminPort = (*StorageAdmin)(nil)
