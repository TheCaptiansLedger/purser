package sql_test

import (
	"path/filepath"
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/datastore/datastoretest"
	"testing"

	sqlstore "purser/internal/adapters/datastore/sql"
)

func TestStore_DatastoreContract(t *testing.T) {
	datastoretest.TestDatastore(t, newTestStore)
}

func newTestStore(t *testing.T) datastore.Datastore {
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

	store, err := sqlstore.New("test", db, sqlstore.DialectSQLite)
	if err != nil {
		t.Fatalf("sqlstore.New returned error: %v", err)
	}
	return store
}
