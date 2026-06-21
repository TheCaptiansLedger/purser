package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImagePath_ShardedLayout(t *testing.T) {
	got := ImagePath("/base", "entries", "abc123", ".jpg")
	if got != "/base/entries/ab/abc123.jpg" {
		t.Errorf("ImagePath = %q, want /base/entries/ab/abc123.jpg", got)
	}
}

func TestImagePath_ShortID(t *testing.T) {
	got := ImagePath("/base", "entries", "a", ".jpg")
	if got != "/base/entries/a/a.jpg" {
		t.Errorf("ImagePath = %q, want /base/entries/a/a.jpg", got)
	}
}

func TestImagePath_NoExt(t *testing.T) {
	got := ImagePath("/base", "entries", "abc123", "")
	if got != "/base/entries/ab/abc123" {
		t.Errorf("ImagePath = %q, want /base/entries/ab/abc123", got)
	}
}

func TestEnsureDirs_CreatesSubdirs(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDirs(dir); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	for _, sub := range []string{"entries", "items", "people", "groups"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("subdir %q missing: %v", sub, err)
		}
	}
}

func TestMigrateFlat_MovesFiles(t *testing.T) {
	dir := t.TempDir()
	entriesDir := filepath.Join(dir, "entries")
	if err := os.MkdirAll(entriesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	flat := filepath.Join(entriesDir, "abc123.jpg")
	if err := os.WriteFile(flat, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateFlat(dir); err != nil {
		t.Fatalf("MigrateFlat: %v", err)
	}

	sharded := filepath.Join(entriesDir, "ab", "abc123.jpg")
	if _, err := os.Stat(sharded); err != nil {
		t.Errorf("sharded file missing at %s: %v", sharded, err)
	}
	if _, err := os.Stat(flat); err == nil {
		t.Error("flat file still exists after migration")
	}
}

func TestMigrateFlat_Idempotent(t *testing.T) {
	dir := t.TempDir()
	entriesDir := filepath.Join(dir, "entries")
	if err := os.MkdirAll(entriesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entriesDir, "abc123.jpg"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateFlat(dir); err != nil {
		t.Fatalf("first MigrateFlat: %v", err)
	}
	if err := MigrateFlat(dir); err != nil {
		t.Fatalf("second MigrateFlat: %v", err)
	}
	if _, err := os.Stat(filepath.Join(entriesDir, "ab", "abc123.jpg")); err != nil {
		t.Errorf("sharded file missing after second migration: %v", err)
	}
}

func TestMigrateFlat_MissingDirIsSkipped(t *testing.T) {
	if err := MigrateFlat(t.TempDir()); err != nil {
		t.Fatalf("MigrateFlat with missing dirs: %v", err)
	}
}
