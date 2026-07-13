package local_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"purser/internal/adapters/imagestore/local"
	"purser/internal/ports"
	"purser/internal/ports/imagestoretest"
	"testing"
)

func TestStore_ImageStoreContract(t *testing.T) {
	imagestoretest.TestImageStore(t, func(t *testing.T) ports.ImageStore {
		return newStore(t)
	})
}

func TestStore_PutRejectsOversizedInputWithoutLeavingAPartialFile(t *testing.T) {
	dir := t.TempDir()
	s, err := local.New("test", dir, local.WithMaxBytes(4))
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}

	_, err = s.Put(context.Background(), "person", "p1", bytes.NewReader([]byte("way too big")))
	if err == nil {
		t.Fatal("Put with oversized input did not return an error")
	}

	var leftover []string
	if walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			leftover = append(leftover, path)
		}
		return nil
	}); walkErr != nil {
		t.Fatalf("WalkDir returned error: %v", walkErr)
	}
	if len(leftover) != 0 {
		t.Fatalf("Put with oversized input left files behind: %v", leftover)
	}
}

func TestStore_GetRejectsKeyEscapingRoot(t *testing.T) {
	dir := t.TempDir()
	s, err := local.New("test", dir)
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}

	if _, err := s.Get(context.Background(), "../../../../etc/passwd"); err == nil {
		t.Fatal("Get with a path-traversal key did not return an error")
	}
}

func TestStore_DeleteRejectsKeyEscapingRoot(t *testing.T) {
	dir := t.TempDir()
	s, err := local.New("test", dir)
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}

	if err := s.Delete(context.Background(), "../../../../etc/passwd"); err == nil {
		t.Fatal("Delete with a path-traversal key did not return an error")
	}
}

func TestStore_PutShardsUnderOwnerType(t *testing.T) {
	dir := t.TempDir()
	s, err := local.New("test", dir)
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}

	key, err := s.Put(context.Background(), "person", "abcdef", bytes.NewReader([]byte{0xFF, 0xD8, 0xFF}))
	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	wantPath := filepath.Join(dir, "person", "ab", "abcdef.jpg")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file at %s (from key %q): %v", wantPath, key, err)
	}
}

func newStore(t *testing.T) *local.Store {
	t.Helper()
	s, err := local.New("test", t.TempDir())
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}
	return s
}
