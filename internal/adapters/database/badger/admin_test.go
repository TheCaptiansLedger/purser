package badger_test

import (
	"purser/internal/adapters/database/databaseadmintest"
	"testing"

	dbbadger "purser/internal/adapters/database/badger"

	dsbadger "purser/internal/adapters/datastore/badger"
)

func TestAdmin_DatabaseAdminContract(t *testing.T) {
	databaseadmintest.TestDatabaseAdmin(t, newTestFixture)
}

func newTestFixture(t *testing.T) databaseadmintest.Fixture {
	t.Helper()

	db, err := dsbadger.Open(dsbadger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("dsbadger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})

	ds, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("dsbadger.New returned error: %v", err)
	}

	admin, err := dbbadger.New(db, ds)
	if err != nil {
		t.Fatalf("dbbadger.New returned error: %v", err)
	}

	return databaseadmintest.Fixture{Admin: admin, DS: ds}
}
