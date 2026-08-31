package local_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"purser/internal/adapters/imagestore/local"
	"purser/internal/ports"
	"purser/internal/ports/imagestoretest"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
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

func TestStore_PutRemovesAStaleSiblingWhenContentTypeChanges(t *testing.T) {
	dir := t.TempDir()
	s, err := local.New("test", dir)
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}

	if _, err := s.Put(context.Background(), "person", "abcdef", bytes.NewReader([]byte{0xFF, 0xD8, 0xFF})); err != nil {
		t.Fatalf("first Put returned error: %v", err)
	}
	jpgPath := filepath.Join(dir, "person", "ab", "abcdef.jpg")
	if _, err := os.Stat(jpgPath); err != nil {
		t.Fatalf("expected file at %s after first Put: %v", jpgPath, err)
	}

	// PNG magic bytes: a later Put for the same id with a different
	// sniffed content type must remove the earlier .jpg, not leave it
	// orphaned on disk.
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	key, err := s.Put(context.Background(), "person", "abcdef", bytes.NewReader(png))
	if err != nil {
		t.Fatalf("second Put returned error: %v", err)
	}

	if _, err := os.Stat(jpgPath); !os.IsNotExist(err) {
		t.Fatalf("stale .jpg sibling still exists after content type changed: %v", err)
	}
	pngPath := filepath.Join(dir, "person", "ab", "abcdef.png")
	if _, err := os.Stat(pngPath); err != nil {
		t.Fatalf("expected file at %s (from key %q): %v", pngPath, key, err)
	}
}

func TestStore_WithTracerProvider(t *testing.T) {
	dir := t.TempDir()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	s, err := local.New("test", dir, local.WithTracerProvider(tp))
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}

	if _, err := s.Put(context.Background(), "person", "p1", bytes.NewReader([]byte{0xFF, 0xD8, 0xFF})); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	if len(exporter.GetSpans()) == 0 {
		t.Fatalf("Put produced no spans with a WithTracerProvider Store")
	}
}

func TestStore_WithLoggerAndWithMeterProvider(t *testing.T) {
	dir := t.TempDir()
	s, err := local.New("test", dir,
		local.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		local.WithMeterProvider(metricnoop.NewMeterProvider()))
	if err != nil {
		t.Fatalf("local.New returned error: %v", err)
	}

	if _, err := s.Put(context.Background(), "person", "p1", bytes.NewReader([]byte{0xFF, 0xD8, 0xFF})); err != nil {
		t.Fatalf("Put with a WithLogger/WithMeterProvider Store returned error: %v", err)
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
