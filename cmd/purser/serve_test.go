package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRequiredBinaries_LogsErrorWhenBinaryMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	checkRequiredBinaries(logger, []string{"definitely-not-a-real-binary"})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output not valid JSON: %v (%s)", err, buf.String())
	}
	if entry["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", entry["level"])
	}
	if entry["binary"] != "definitely-not-a-real-binary" {
		t.Errorf("binary = %v, want %q", entry["binary"], "definitely-not-a-real-binary")
	}
}

func TestCheckRequiredBinaries_NoLogWhenBinaryPresent(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fake-ffprobe")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // deliberately executable: LookPath requires the exec bit
		t.Fatalf("os.WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	checkRequiredBinaries(logger, []string{"fake-ffprobe"})

	if buf.Len() != 0 {
		t.Errorf("expected no log output, got: %s", buf.String())
	}
}
