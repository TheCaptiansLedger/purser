package unmatchedfile_test

import (
	"path/filepath"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store/unmatchedfile"
	"purser/internal/ports"
	"purser/internal/ports/unmatchedfiletest"
	"testing"

	dsbadger "purser/internal/adapters/datastore/badger"
	dssql "purser/internal/adapters/datastore/sql"
)

func TestRepository_UnmatchedFileRepositoryContract_Badger(t *testing.T) {
	unmatchedfiletest.TestUnmatchedFileRepository(t, func(t *testing.T) ports.UnmatchedFileRepository {
		return newRepository(t, newBadgerDatastore(t))
	})
}

func TestRepository_UnmatchedFileRepositoryContract_SQL(t *testing.T) {
	unmatchedfiletest.TestUnmatchedFileRepository(t, func(t *testing.T) ports.UnmatchedFileRepository {
		return newRepository(t, newSQLDatastore(t))
	})
}

func newRepository(t *testing.T, ds datastore.Datastore) ports.UnmatchedFileRepository {
	t.Helper()
	repo, err := unmatchedfile.New("test", ds)
	if err != nil {
		t.Fatalf("unmatchedfile.New returned error: %v", err)
	}
	return repo
}

func newBadgerDatastore(t *testing.T) datastore.Datastore {
	t.Helper()

	db, err := dsbadger.Open(dsbadger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("badger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})

	store, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}
	return store
}

func newSQLDatastore(t *testing.T) datastore.Datastore {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := dssql.Open(dssql.Options{Dialect: dssql.DialectSQLite, DSN: dsn})
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})

	store, err := dssql.New("test", db, dssql.DialectSQLite)
	if err != nil {
		t.Fatalf("sql.New returned error: %v", err)
	}
	return store
}
