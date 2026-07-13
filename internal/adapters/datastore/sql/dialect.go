package sql

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

// Dialect names a supported SQL engine family. See
// docs/adr/0012-datastore-persistence.md — the only genuinely
// dialect-specific code in this package is placeholder rewriting
// (rebind) and duplicate-key error detection (isConflict); every SQL
// statement itself is written once.
type Dialect string

// The complete set of supported dialects.
const (
	DialectPostgres Dialect = "postgres"
	DialectMySQL    Dialect = "mysql"
	DialectSQLite   Dialect = "sqlite"
)

// rebind rewrites a query written with `?` placeholders for dialect.
// Postgres requires positional `$1, $2, ...` placeholders; MySQL and
// SQLite already accept `?` and are returned unmodified.
func rebind(query string, dialect Dialect) string {
	if dialect != DialectPostgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	for _, r := range query {
		if r != '?' {
			b.WriteRune(r)
			continue
		}
		n++
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(n))
	}
	return b.String()
}

// isConflict reports whether err is a primary-key/unique-constraint
// violation for dialect — the signal Create maps to
// purser/internal/ports.ErrConflict. Relying on the constraint itself
// (rather than a read-then-insert check) keeps Create race-free under
// concurrent writers.
func isConflict(dialect Dialect, err error) bool {
	if err == nil {
		return false
	}
	switch dialect {
	case DialectPostgres:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr.Code == "23505"
		}
	case DialectMySQL:
		var myErr *mysql.MySQLError
		if errors.As(err, &myErr) {
			return myErr.Number == 1062
		}
	case DialectSQLite:
		return strings.Contains(err.Error(), "UNIQUE constraint failed")
	}
	return false
}
