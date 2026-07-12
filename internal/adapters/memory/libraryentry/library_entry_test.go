package libraryentry_test

import (
	"purser/internal/ports"
	"purser/internal/ports/libraryentrytest"
	"testing"

	memlibraryentry "purser/internal/adapters/memory/libraryentry"
)

func TestRepository_Contract(t *testing.T) {
	libraryentrytest.TestLibraryEntryRepository(t, func(t *testing.T) ports.LibraryEntryRepository {
		t.Helper()
		r, err := memlibraryentry.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memlibraryentry.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
