package mediafile_test

import (
	"purser/internal/ports"
	"purser/internal/ports/mediafiletest"
	"testing"

	memmediafile "purser/internal/adapters/memory/mediafile"
)

func TestRepository_Contract(t *testing.T) {
	mediafiletest.TestMediaFileRepository(t, func(t *testing.T) ports.MediaFileRepository {
		t.Helper()
		r, err := memmediafile.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memmediafile.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
