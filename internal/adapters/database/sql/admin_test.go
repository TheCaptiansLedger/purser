package sql_test

import (
	"path/filepath"
	"purser/internal/adapters/database/databaseadmintest"
	"testing"

	dbsql "purser/internal/adapters/database/sql"
	sqlstore "purser/internal/adapters/datastore/sql"
)

func TestAdmin_DatabaseAdminContract(t *testing.T) {
	databaseadmintest.TestDatabaseAdmin(t, newTestFixture)
}

func newTestFixture(t *testing.T) databaseadmintest.Fixture {
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

	ds, err := sqlstore.New("test", db, sqlstore.DialectSQLite)
	if err != nil {
		t.Fatalf("sqlstore.New returned error: %v", err)
	}

	admin, err := dbsql.New(db, sqlstore.DialectSQLite, ds)
	if err != nil {
		t.Fatalf("dbsql.New returned error: %v", err)
	}

	return databaseadmintest.Fixture{Admin: admin, DS: ds}
}
