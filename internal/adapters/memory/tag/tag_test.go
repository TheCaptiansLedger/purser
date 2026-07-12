package tag_test

import (
	"purser/internal/ports"
	"purser/internal/ports/tagtest"
	"testing"

	memtag "purser/internal/adapters/memory/tag"
)

func TestRepository_Contract(t *testing.T) {
	tagtest.TestTagRepository(t, func(t *testing.T) ports.TagRepository {
		t.Helper()
		r, err := memtag.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memtag.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
