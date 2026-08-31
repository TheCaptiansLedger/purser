package sql

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRebind(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		dialect Dialect
		want    string
	}{
		{"postgres rewrites every placeholder positionally", "SELECT ? FROM t WHERE a = ? AND b = ?", DialectPostgres, "SELECT $1 FROM t WHERE a = $2 AND b = $3"},
		{"postgres with no placeholders is unchanged", "SELECT 1", DialectPostgres, "SELECT 1"},
		{"mysql is returned unmodified", "SELECT ? FROM t WHERE a = ?", DialectMySQL, "SELECT ? FROM t WHERE a = ?"},
		{"sqlite is returned unmodified", "SELECT ? FROM t WHERE a = ?", DialectSQLite, "SELECT ? FROM t WHERE a = ?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rebind(tt.query, tt.dialect); got != tt.want {
				t.Fatalf("rebind(%q, %q) = %q, want %q", tt.query, tt.dialect, got, tt.want)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		err     error
		want    bool
	}{
		{"nil error is never a conflict", DialectPostgres, nil, false},
		{"postgres unique violation is a conflict", DialectPostgres, &pgconn.PgError{Code: "23505"}, true},
		{"postgres other error code is not a conflict", DialectPostgres, &pgconn.PgError{Code: "42601"}, false},
		{"postgres non-pg error is not a conflict", DialectPostgres, errors.New("boom"), false},
		{"mysql duplicate entry is a conflict", DialectMySQL, &mysql.MySQLError{Number: 1062}, true},
		{"mysql other error number is not a conflict", DialectMySQL, &mysql.MySQLError{Number: 1045}, false},
		{"mysql non-mysql error is not a conflict", DialectMySQL, errors.New("boom"), false},
		{"sqlite unique constraint message is a conflict", DialectSQLite, errors.New("UNIQUE constraint failed: documents.id"), true},
		{"sqlite other message is not a conflict", DialectSQLite, errors.New("no such table: documents"), false},
		{"unknown dialect is never a conflict", Dialect("unknown"), &pgconn.PgError{Code: "23505"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConflict(tt.dialect, tt.err); got != tt.want {
				t.Fatalf("isConflict(%q, %v) = %v, want %v", tt.dialect, tt.err, got, tt.want)
			}
		})
	}
}
