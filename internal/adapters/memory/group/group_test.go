package group_test

import (
	"purser/internal/ports"
	"purser/internal/ports/grouptest"
	"testing"

	memgroup "purser/internal/adapters/memory/group"
)

func TestRepository_Contract(t *testing.T) {
	grouptest.TestGroupRepository(t, func(t *testing.T) ports.GroupRepository {
		t.Helper()
		r, err := memgroup.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memgroup.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
