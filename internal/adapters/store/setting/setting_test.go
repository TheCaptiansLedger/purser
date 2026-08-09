package setting_test

import (
	"path/filepath"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/store/setting"
	"purser/internal/ports"
	"purser/internal/ports/settingtest"
	"testing"

	dsbadger "purser/internal/adapters/datastore/badger"
	dssql "purser/internal/adapters/datastore/sql"
)

func TestRepository_SettingsRepositoryContract_Badger(t *testing.T) {
	settingtest.TestSettingsRepository(t, func(t *testing.T) ports.SettingsRepository {
		return newRepository(t, newBadgerDatastore(t))
	})
}

func TestRepository_SettingsRepositoryContract_SQL(t *testing.T) {
	settingtest.TestSettingsRepository(t, func(t *testing.T) ports.SettingsRepository {
		return newRepository(t, newSQLDatastore(t))
	})
}

func newRepository(t *testing.T, ds datastore.Datastore) ports.SettingsRepository {
	t.Helper()
	repo, err := setting.New("test", ds)
	if err != nil {
		t.Fatalf("setting.New returned error: %v", err)
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
