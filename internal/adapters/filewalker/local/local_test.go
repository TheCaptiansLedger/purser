package local_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"purser/internal/adapters/filewalker/local"
	"purser/internal/ports"
	"purser/internal/ports/filewalkertest"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestWalker_FileWalkerContract(t *testing.T) {
	filewalkertest.TestFileWalker(t, func(t *testing.T) ports.FileWalker {
		t.Helper()
		w, err := local.New()
		if err != nil {
			t.Fatalf("local.New returned error: %v", err)
		}
		return w
	})
}

func TestNew_WithOptions(t *testing.T) {
	w, err := local.New(
		local.WithLogger(slog.Default()),
		local.WithTracerProvider(otel.GetTracerProvider()),
		local.WithMeterProvider(otel.GetMeterProvider()),
	)
	if err != nil {
		t.Fatalf("local.New with options returned error: %v", err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Walk returned %d files, want 1", len(files))
	}
}
