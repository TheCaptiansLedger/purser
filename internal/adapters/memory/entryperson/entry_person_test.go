package entryperson_test

import (
	"purser/internal/ports"
	"purser/internal/ports/entrypersontest"
	"testing"

	preentryperson "purser/internal/adapters/memory/entryperson"
)

func TestRepository_Contract(t *testing.T) {
	entrypersontest.TestEntryPersonRepository(t, func(t *testing.T) ports.EntryPersonRepository {
		t.Helper()
		r, err := preentryperson.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := preentryperson.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
