package badger_test

import (
	"purser/internal/adapters/datastore"
	"purser/internal/adapters/datastore/badger"
	"purser/internal/adapters/datastore/datastoretest"
	"testing"
)

func TestStore_DatastoreContract(t *testing.T) {
	datastoretest.TestDatastore(t, newTestStore)
}

func newTestStore(t *testing.T) datastore.Datastore {
	t.Helper()

	db, err := badger.Open(badger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("badger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})

	store, err := badger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}
	return store
}
