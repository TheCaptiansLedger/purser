package fingerprint_test

import (
	"context"
	"os"
	"purser/internal/app/errs"
	"purser/internal/ports"
	"testing"
)

type stubFS struct {
	hash string
	err  error
}

func (s *stubFS) OSHash(_ context.Context, _ string) (string, error) { return s.hash, s.err }
func (s *stubFS) Stat(_ context.Context, _ string) (*ports.FileInfo, error) {
	return nil, errs.ErrNotFound
}
func (s *stubFS) Move(_ context.Context, _, _ string) error                            { return nil }
func (s *stubFS) Walk(_ context.Context, _ string, _ func(ports.FileInfo) error) error { return nil }

func writeTempFile(t *testing.T, ext string, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*"+ext)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}
