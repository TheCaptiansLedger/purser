package image_test

import (
	"purser/internal/ports"
	"purser/internal/ports/imagetest"
	"testing"

	memimage "purser/internal/adapters/memory/image"
)

func TestRepository_Contract(t *testing.T) {
	imagetest.TestImageRepository(t, func(t *testing.T) ports.ImageRepository {
		t.Helper()
		r, err := memimage.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memimage.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
