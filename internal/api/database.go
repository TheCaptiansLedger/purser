package api

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type databaseHandler struct {
	db         *sql.DB
	dsn        string
	shutdownFn func()
}

func (h *databaseHandler) routes(r chi.Router) {
	r.Get("/stats", h.stats)
	r.Get("/backup", h.backup)
	r.Post("/restore", h.restore)
}

type tableStats struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

type dbStatsResponse struct {
	Tables         []tableStats `json:"tables"`
	FileSizeBytes  int64        `json:"file_size_bytes"`
	SQLiteVersion  string       `json:"sqlite_version"`
	MigrationCount int          `json:"migration_count"`
}

type restoreResponse struct {
	Message   string       `json:"message"`
	Tables    []tableStats `json:"tables"`
	TotalRows int64        `json:"total_rows"`
}

func (h *databaseHandler) stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var version string
	_ = h.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)

	rows, err := h.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_ERROR", "failed to list tables")
		return
	}
	defer func() { _ = rows.Close() }()

	tables, total := scanTableStats(ctx, rows, h.db)
	_ = total

	var fileSize int64
	if info, err := os.Stat(h.dsn); err == nil {
		fileSize = info.Size()
	}

	var migCount int
	_ = h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migCount)

	writeJSON(w, http.StatusOK, dbStatsResponse{
		Tables:         tables,
		FileSizeBytes:  fileSize,
		SQLiteVersion:  version,
		MigrationCount: migCount,
	})
}

// backup streams the database as a SQL dump.
func (h *databaseHandler) backup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="purser.sql"`)

	bw := bufio.NewWriter(w)
	defer bw.Flush() //nolint:errcheck

	_, _ = fmt.Fprintf(bw, "-- Purser database dump\n-- Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	_, _ = fmt.Fprintln(bw, "PRAGMA foreign_keys=OFF;")
	_, _ = fmt.Fprintln(bw, "BEGIN TRANSACTION;")

	schemaRows, err := h.db.QueryContext(ctx,
		`SELECT type, name, sql FROM sqlite_master
		 WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'
		 ORDER BY rootpage`)
	if err != nil {
		return
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
	_ = schemaRows.Close()

	for _, table := range tables {
		dataRows, err := h.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %q", table))
		if err != nil {
			continue
		}
		cols, _ := dataRows.Columns()
		for dataRows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := dataRows.Scan(ptrs...); err != nil {
				continue
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
		_ = dataRows.Close()
	}

	_, _ = fmt.Fprintln(bw, "\nCOMMIT;")
	_, _ = fmt.Fprintln(bw, "PRAGMA foreign_keys=ON;")
}

// restore accepts a SQL dump, applies it to a fresh DB, validates it, swaps it in, and exits
// so the container restarts with the new database.
func (h *databaseHandler) restore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil { //nolint:gosec
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "failed to parse upload")
		return
	}

	file, _, err := r.FormFile("database")
	if err != nil {
		writeError(w, http.StatusBadRequest, "FILE_MISSING", "database file is required")
		return
	}
	defer func() { _ = file.Close() }()

	sqlContent, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "READ_ERROR", "failed to read upload")
		return
	}

	// Build a fresh SQLite database at a temp path in the same directory so rename is atomic.
	tmp, err := os.CreateTemp(filepath.Dir(h.dsn), "purser-restore-*.db")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TEMP_ERROR", "failed to create temp file")
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }() // no-op if rename succeeds

	tmpDB, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_OPEN_ERROR", "failed to open temp database")
		return
	}

	for _, stmt := range splitSQL(string(sqlContent)) {
		if _, err := tmpDB.ExecContext(r.Context(), stmt); err != nil {
			_ = tmpDB.Close()
			writeError(w, http.StatusBadRequest, "SQL_ERROR", fmt.Sprintf("SQL execution failed: %v", err))
			return
		}
	}

	// Validate that this is a Purser database.
	var migCount int
	if err := tmpDB.QueryRowContext(
		r.Context(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'",
	).Scan(&migCount); err != nil || migCount == 0 {
		_ = tmpDB.Close()
		writeError(w, http.StatusBadRequest, "INVALID_DB", "not a valid Purser database (schema_migrations missing)")
		return
	}

	tables, totalRows := collectTableStats(r.Context(), tmpDB)
	_ = tmpDB.Close()

	_, _ = h.db.ExecContext(r.Context(), "PRAGMA wal_checkpoint(TRUNCATE)")

	if err := os.Rename(tmpPath, h.dsn); err != nil {
		writeError(w, http.StatusInternalServerError, "REPLACE_ERROR", "failed to replace database")
		return
	}

	writeJSON(w, http.StatusOK, restoreResponse{
		Message:   "Database restored successfully. Server is restarting.",
		Tables:    tables,
		TotalRows: totalRows,
	})

	// Signal the application lifecycle to shut down cleanly. A process supervisor
	// (systemd, Docker restart policy, k8s) will restart the process to load
	// the restored database file.
	go h.shutdownFn()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanTableStats(ctx context.Context, nameRows *sql.Rows, db *sql.DB) ([]tableStats, int64) {
	var tables []tableStats
	var total int64
	for nameRows.Next() {
		var name string
		if err := nameRows.Scan(&name); err != nil {
			continue
		}
		var count int64
		_ = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", name)).Scan(&count)
		tables = append(tables, tableStats{Name: name, Rows: count})
		total += count
	}
	if tables == nil {
		tables = []tableStats{}
	}
	return tables, total
}

func collectTableStats(ctx context.Context, db *sql.DB) ([]tableStats, int64) {
	rows, err := db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, 0
	}
	defer func() { _ = rows.Close() }()
	return scanTableStats(ctx, rows, db)
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

// splitSQL splits a SQL dump into individual statements, correctly handling
// single-quoted strings (with ” escaping) and -- line comments.
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
