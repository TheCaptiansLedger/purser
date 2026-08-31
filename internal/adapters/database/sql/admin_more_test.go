package sql_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	dbsql "purser/internal/adapters/database/sql"
	sqlstore "purser/internal/adapters/datastore/sql"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// openTestDB returns a fresh SQLite-backed *sql.DB/datastore.Datastore
// pair, plus the raw db handle a test needs to sabotage the schema
// directly (dropping a table) to exercise an error path a healthy
// database never takes.
func openTestDB(t *testing.T) (*sql.DB, sqlstore.Dialect) {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlstore.Open(sqlstore.Options{Dialect: sqlstore.DialectSQLite, DSN: dsn})
	if err != nil {
		t.Fatalf("sqlstore.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})
	return db, sqlstore.DialectSQLite
}

func newTestDS(t *testing.T, db *sql.DB, dialect sqlstore.Dialect) *sqlstore.Store {
	t.Helper()
	ds, err := sqlstore.New("test", db, dialect)
	if err != nil {
		t.Fatalf("sqlstore.New returned error: %v", err)
	}
	return ds
}

func TestNew_RejectsNilDB(t *testing.T) {
	if _, err := dbsql.New(nil, sqlstore.DialectSQLite, nil); err == nil {
		t.Fatalf("New with a nil db returned a nil error")
	}
}

func TestNew_RejectsNilDatastore(t *testing.T) {
	db, dialect := openTestDB(t)
	_, err := dbsql.New(db, dialect, nil)
	if err == nil {
		t.Fatalf("New with a nil datastore returned a nil error")
	}
}

func TestWithTracerProvider(t *testing.T) {
	db, dialect := openTestDB(t)
	ds := newTestDS(t, db, dialect)

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	admin, err := dbsql.New(db, dialect, ds, dbsql.WithTracerProvider(tp))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := admin.Info(context.Background()); err != nil {
		t.Fatalf("Info returned error: %v", err)
	}

	if len(exporter.GetSpans()) == 0 {
		t.Fatalf("Info produced no spans with a WithTracerProvider Admin")
	}
}

func TestWithLogger(t *testing.T) {
	db, dialect := openTestDB(t)
	ds := newTestDS(t, db, dialect)

	admin, err := dbsql.New(db, dialect, ds, dbsql.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, err := admin.Info(context.Background()); err != nil {
		t.Fatalf("Info with a WithLogger Admin returned error: %v", err)
	}
}

func TestInfo_UnknownDialectReportsUnknownVersionAndErrorsOnStorageSize(t *testing.T) {
	db, _ := openTestDB(t)
	ds := newTestDS(t, db, sqlstore.DialectSQLite)

	admin, err := dbsql.New(db, sqlstore.Dialect("unknown"), ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := admin.Info(context.Background()); err == nil {
		t.Fatalf("Info with an unknown dialect returned a nil error, want storageSize's unknown-dialect error")
	}
}

func TestInfo_MismatchedDialectSurfacesTheDialectSpecificQueryError(t *testing.T) {
	// A real SQLite database doesn't implement Postgres's/MySQL's
	// version()/pg_database_size()/information_schema queries — pointing
	// an Admin declared as that dialect at the SQLite fixture exercises
	// version's and storageSize's dialect-specific branches and confirms
	// each one's query error is propagated, not swallowed.
	for _, dialect := range []sqlstore.Dialect{sqlstore.DialectPostgres, sqlstore.DialectMySQL} {
		t.Run(string(dialect), func(t *testing.T) {
			db, _ := openTestDB(t)
			ds := newTestDS(t, db, sqlstore.DialectSQLite)

			admin, err := dbsql.New(db, dialect, ds)
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			if _, err := admin.Info(context.Background()); err == nil {
				t.Fatalf("Info for a %s-declared Admin against a SQLite database returned a nil error", dialect)
			}
		})
	}
}

func TestBackup_PropagatesAWriteHeaderError(t *testing.T) {
	db, dialect := openTestDB(t)
	ds := newTestDS(t, db, dialect)
	admin, err := dbsql.New(db, dialect, ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := admin.Backup(context.Background(), failingWriter{}); err == nil {
		t.Fatalf("Backup with a failing writer returned a nil error")
	}
}

func TestRestore_PropagatesAClearErrorWhenDocumentIndexIsMissing(t *testing.T) {
	db, dialect := openTestDB(t)
	ds := newTestDS(t, db, dialect)
	admin, err := dbsql.New(db, dialect, ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := db.Exec("DROP TABLE document_index"); err != nil {
		t.Fatalf("dropping document_index: %v", err)
	}

	var buf bytes.Buffer
	if err := admin.Backup(context.Background(), &buf); err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}

	if err := admin.Restore(context.Background(), bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatalf("Restore with document_index dropped returned a nil error")
	}
}

func TestRestore_PropagatesAClearErrorWhenDocumentsIsMissing(t *testing.T) {
	db, dialect := openTestDB(t)
	ds := newTestDS(t, db, dialect)
	admin, err := dbsql.New(db, dialect, ds)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := admin.Backup(context.Background(), &buf); err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}

	if _, err := db.Exec("DROP TABLE documents"); err != nil {
		t.Fatalf("dropping documents: %v", err)
	}

	if err := admin.Restore(context.Background(), bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatalf("Restore with documents dropped returned a nil error")
	}
}

// failingWriter always returns an error from Write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }
