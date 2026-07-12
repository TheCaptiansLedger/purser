package itemperson_test

import (
	"purser/internal/ports"
	"purser/internal/ports/itempersontest"
	"testing"

	memitemperson "purser/internal/adapters/memory/itemperson"
)

func TestRepository_Contract(t *testing.T) {
	itempersontest.TestItemPersonRepository(t, func(t *testing.T) ports.ItemPersonRepository {
		t.Helper()
		r, err := memitemperson.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memitemperson.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
