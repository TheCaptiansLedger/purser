package person_test

import (
	"purser/internal/ports"
	"purser/internal/ports/persontest"
	"testing"

	memperson "purser/internal/adapters/memory/person"
)

func TestRepository_Contract(t *testing.T) {
	persontest.TestPersonRepository(t, func(t *testing.T) ports.PersonRepository {
		t.Helper()
		r, err := memperson.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memperson.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
