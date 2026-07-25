// Package filewalkertest is the shared contract test suite for the
// ports.FileWalker port. See internal/ports/imagestoretest for the
// convention this follows — a contract test exists even though
// internal/adapters/filewalker/local is FileWalker's only implementation
// today, the same precedent imagestoretest/imagestore-local already sets.
package filewalkertest

import (
	"context"
	"os"
	"path/filepath"
	"purser/internal/ports"
	"runtime"
	"sort"
	"testing"
)

// NewFileWalkerFunc returns a fresh ports.FileWalker for the duration of a
// single subtest.
type NewFileWalkerFunc func(t *testing.T) ports.FileWalker

// TestFileWalker runs the shared FileWalker contract against newWalker.
func TestFileWalker(t *testing.T, newWalker NewFileWalkerFunc) {
	t.Helper()

	t.Run("walk an empty directory returns no files", func(t *testing.T) { testWalkEmpty(t, newWalker) })
	t.Run("walk a nested tree returns every regular file with its size", func(t *testing.T) { testWalkNested(t, newWalker) })
	t.Run("walk skips subdirectories themselves", func(t *testing.T) { testWalkSkipsDirs(t, newWalker) })
	t.Run("walk a nonexistent root returns an error", func(t *testing.T) { testWalkNonexistentRoot(t, newWalker) })
	t.Run("walk skips symlinks", func(t *testing.T) { testWalkSkipsSymlinks(t, newWalker) })
}

func testWalkEmpty(t *testing.T, newWalker NewFileWalkerFunc) {
	w := newWalker(t)
	root := t.TempDir()

	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("Walk(empty dir) returned %d files, want 0", len(files))
	}
}

func testWalkNested(t *testing.T, newWalker NewFileWalkerFunc) {
	w := newWalker(t)
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "top.txt"), "top")
	mustMkdirAll(t, filepath.Join(root, "sub", "deeper"))
	mustWriteFile(t, filepath.Join(root, "sub", "middle.txt"), "middle")
	mustWriteFile(t, filepath.Join(root, "sub", "deeper", "bottom.txt"), "bottommost")

	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("Walk returned %d files, want 3: %+v", len(files), files)
	}

	got := map[string]int64{}
	for _, f := range files {
		got[f.Path] = f.Size
	}

	want := map[string]int64{
		filepath.Join(root, "top.txt"):                     int64(len("top")),
		filepath.Join(root, "sub", "middle.txt"):           int64(len("middle")),
		filepath.Join(root, "sub", "deeper", "bottom.txt"): int64(len("bottommost")),
	}
	for path, size := range want {
		gotSize, ok := got[path]
		if !ok {
			t.Errorf("Walk result missing %q", path)
			continue
		}
		if gotSize != size {
			t.Errorf("Walk(%q).Size = %d, want %d", path, gotSize, size)
		}
	}
}

func testWalkSkipsDirs(t *testing.T, newWalker NewFileWalkerFunc) {
	w := newWalker(t)
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "empty-subdir"))
	mustWriteFile(t, filepath.Join(root, "file.txt"), "x")

	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)

	want := []string{filepath.Join(root, "file.txt")}
	if len(paths) != len(want) || paths[0] != want[0] {
		t.Fatalf("Walk returned paths %v, want %v", paths, want)
	}
}

func testWalkNonexistentRoot(t *testing.T, newWalker NewFileWalkerFunc) {
	w := newWalker(t)
	root := filepath.Join(t.TempDir(), "does-not-exist")

	if _, err := w.Walk(context.Background(), root); err == nil {
		t.Fatal("Walk(nonexistent root): expected error, got nil")
	}
}

func testWalkSkipsSymlinks(t *testing.T, newWalker NewFileWalkerFunc) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on windows")
	}

	w := newWalker(t)
	root := t.TempDir()
	target := filepath.Join(root, "real.txt")
	mustWriteFile(t, target, "real")

	if err := os.Symlink(target, filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	if len(files) != 1 || files[0].Path != target {
		t.Fatalf("Walk returned %+v, want exactly [%q]", files, target)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q): %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", path, err)
	}
}
