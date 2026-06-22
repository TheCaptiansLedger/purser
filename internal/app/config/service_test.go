package config_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"purser/internal/app/errs"
	"strings"
	"testing"

	"github.com/spf13/viper"

	appconfig "purser/internal/app/config"

	internalconfig "purser/internal/config"
)

// stubSettings is an in-memory SettingsRepository for tests.
type stubSettings struct {
	data map[string]string
}

func newStubSettings() *stubSettings {
	return &stubSettings{data: make(map[string]string)}
}

func (s *stubSettings) Get(_ context.Context, key string) (string, error) {
	v, ok := s.data[key]
	if !ok {
		return "", errs.ErrNotFound
	}
	return v, nil
}

func (s *stubSettings) Set(_ context.Context, key, value string) error {
	s.data[key] = value
	return nil
}

// clearPurserEnv unsets every PURSER_* environment variable for the duration
// of the test, preventing ambient host config from affecting locking detection.
func clearPurserEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(k, "PURSER_") {
			continue
		}
		orig := os.Getenv(k)
		os.Unsetenv(k)
		t.Cleanup(func() { os.Setenv(k, orig) })
	}
}

// loadViperFull calls LoadFull and returns the viper instance and locked set.
func loadViperFull(t *testing.T, path string) (*viper.Viper, map[string]struct{}) {
	t.Helper()
	_, v, locked, err := internalconfig.LoadFull(path)
	if err != nil {
		t.Fatalf("LoadFull: %v", err)
	}
	return v, locked
}

func TestIsLocked_EnvSet(t *testing.T) {
	clearPurserEnv(t)
	t.Setenv("PURSER_SOURCES_TMDB_API_KEY", "env-token")
	v, locked := loadViperFull(t, "")
	svc := appconfig.New(v, locked, newStubSettings())
	if !svc.IsLocked("sources.tmdb.api_key") {
		t.Error("key set via env var should be locked")
	}
}

func TestIsLocked_FileSet(t *testing.T) {
	clearPurserEnv(t)
	f := filepath.Join(t.TempDir(), "purser.yaml")
	if err := os.WriteFile(f, []byte("sources:\n  tmdb:\n    api_key: file-token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, locked := loadViperFull(t, f)
	svc := appconfig.New(v, locked, newStubSettings())
	if !svc.IsLocked("sources.tmdb.api_key") {
		t.Error("key set via YAML file should be locked")
	}
}

func TestIsLocked_DBOnly(t *testing.T) {
	clearPurserEnv(t)
	v, locked := loadViperFull(t, "")
	stub := newStubSettings()
	_ = stub.Set(context.Background(), "sources.tmdb.api_key", "db-token")
	svc := appconfig.New(v, locked, stub)
	if svc.IsLocked("sources.tmdb.api_key") {
		t.Error("key present only in DB should not be locked")
	}
}

func TestGet_Precedence_EnvOverDB(t *testing.T) {
	clearPurserEnv(t)
	t.Setenv("PURSER_SOURCES_TMDB_API_KEY", "env-token")
	v, locked := loadViperFull(t, "")
	stub := newStubSettings()
	_ = stub.Set(context.Background(), "sources.tmdb.api_key", "db-token")
	svc := appconfig.New(v, locked, stub)

	got, err := svc.Get(context.Background(), "sources.tmdb.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "env-token" {
		t.Errorf("Get = %q, want env-token", got)
	}
}

func TestGet_Precedence_DBOverDefault(t *testing.T) {
	clearPurserEnv(t)
	v, locked := loadViperFull(t, "")
	stub := newStubSettings()
	_ = stub.Set(context.Background(), "sources.tmdb.api_key", "db-token")
	svc := appconfig.New(v, locked, stub)

	got, err := svc.Get(context.Background(), "sources.tmdb.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "db-token" {
		t.Errorf("Get = %q, want db-token", got)
	}
}

func TestGet_DefaultFallback(t *testing.T) {
	clearPurserEnv(t)
	v, locked := loadViperFull(t, "")
	svc := appconfig.New(v, locked, newStubSettings())

	got, err := svc.Get(context.Background(), "log.level")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "info" {
		t.Errorf("Get = %q, want info (built-in default)", got)
	}
}

func TestSet_LockedReturnsErrLocked(t *testing.T) {
	clearPurserEnv(t)
	t.Setenv("PURSER_SOURCES_TMDB_API_KEY", "env-token")
	v, locked := loadViperFull(t, "")
	svc := appconfig.New(v, locked, newStubSettings())

	err := svc.Set(context.Background(), "sources.tmdb.api_key", "new-value")
	if err == nil {
		t.Fatal("Set on locked key should return an error")
	}
	if !errors.Is(err, errs.ErrLocked) {
		t.Errorf("Set error = %v, want errs.ErrLocked", err)
	}
}

func TestSet_WritesToDB(t *testing.T) {
	clearPurserEnv(t)
	v, locked := loadViperFull(t, "")
	stub := newStubSettings()
	svc := appconfig.New(v, locked, stub)

	if err := svc.Set(context.Background(), "sources.tmdb.api_key", "new-token"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := stub.Get(context.Background(), "sources.tmdb.api_key")
	if err != nil {
		t.Fatalf("stub.Get: %v", err)
	}
	if got != "new-token" {
		t.Errorf("stub value = %q, want new-token", got)
	}
}
