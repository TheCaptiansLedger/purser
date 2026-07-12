package performerprofile_test

import (
	"purser/internal/ports"
	"purser/internal/ports/performerprofiletest"
	"testing"

	memperformerprofile "purser/internal/adapters/memory/performerprofile"
)

func TestRepository_Contract(t *testing.T) {
	performerprofiletest.TestPerformerProfileRepository(t, func(t *testing.T) ports.PerformerProfileRepository {
		t.Helper()
		r, err := memperformerprofile.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memperformerprofile.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
