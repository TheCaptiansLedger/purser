package sql_test

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"purser/internal/adapters/datastore"
	"testing"

	sqlstore "purser/internal/adapters/datastore/sql"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOpen_RejectsAnUnknownDialect(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "test.db")
	if _, err := sqlstore.Open(sqlstore.Options{Dialect: sqlstore.Dialect("unknown"), DSN: dsn}); err == nil {
		t.Fatalf("Open with an unknown dialect returned a nil error")
	}
}

func TestOpen_SkipsAlreadyAppliedMigrationsOnASecondOpen(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "test.db")

	db1, err := sqlstore.Open(sqlstore.Options{Dialect: sqlstore.DialectSQLite, DSN: dsn})
	if err != nil {
		t.Fatalf("first Open returned error: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("closing first db: %v", err)
	}

	// Re-opening the same DSN re-runs runMigrations against a database
	// that already has every migration recorded in schema_migrations —
	// exercising the "already applied, skip" branch rather than the
	// "apply for the first time" branch every other test takes.
	db2, err := sqlstore.Open(sqlstore.Options{Dialect: sqlstore.DialectSQLite, DSN: dsn})
	if err != nil {
		t.Fatalf("second Open returned error: %v", err)
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("closing second db: %v", err)
	}
}

func TestNew_RejectsEmptyName(t *testing.T) {
	db, err := sqlstore.Open(sqlstore.Options{Dialect: sqlstore.DialectSQLite, DSN: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := sqlstore.New("", db, sqlstore.DialectSQLite); err == nil {
		t.Fatalf("New with an empty name returned a nil error")
	}
}

func TestNew_RejectsNilDB(t *testing.T) {
	if _, err := sqlstore.New("test", nil, sqlstore.DialectSQLite); err == nil {
		t.Fatalf("New with a nil db returned a nil error")
	}
}

func TestNew_WithTracerProvider(t *testing.T) {
	db, err := sqlstore.Open(sqlstore.Options{Dialect: sqlstore.DialectSQLite, DSN: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() { _ = db.Close() }()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	store, err := sqlstore.New("test", db, sqlstore.DialectSQLite, sqlstore.WithTracerProvider(tp))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := store.Create(context.Background(), datastore.Document{Collection: "widget", ID: "trace", Data: []byte(`{}`)}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if len(exporter.GetSpans()) == 0 {
		t.Fatalf("Create produced no spans with a WithTracerProvider Store")
	}
}

func TestNew_WithLogger(t *testing.T) {
	db, err := sqlstore.Open(sqlstore.Options{Dialect: sqlstore.DialectSQLite, DSN: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() { _ = db.Close() }()

	store, err := sqlstore.New("test", db, sqlstore.DialectSQLite, sqlstore.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := store.Create(context.Background(), datastore.Document{Collection: "widget", ID: "logger", Data: []byte(`{}`)}); err != nil {
		t.Fatalf("Create with a WithLogger Store returned error: %v", err)
	}
}
