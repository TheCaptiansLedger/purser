package item_test

import (
	"purser/internal/ports"
	"purser/internal/ports/itemtest"
	"testing"

	memitem "purser/internal/adapters/memory/item"
)

func TestRepository_Contract(t *testing.T) {
	itemtest.TestItemRepository(t, func(t *testing.T) ports.ItemRepository {
		t.Helper()
		r, err := memitem.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memitem.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
