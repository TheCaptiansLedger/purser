package contract

import (
	"context"
	"purser/internal/app/errs"
	"testing"
)

func runSettingsContract(t *testing.T, s BackendSuite) {
	t.Helper()

	t.Run("SetAndGet", func(t *testing.T) {
		ctx := context.Background()
		if err := s.Settings.Set(ctx, "theme", "dark"); err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := s.Settings.Get(ctx, "theme")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != "dark" {
			t.Errorf("Get = %q, want dark", got)
		}
	})

	t.Run("GetMissingKey", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.Settings.Get(ctx, "key-that-does-not-exist-xyz")
		if !errs.IsNotFound(err) {
			t.Errorf("Get missing key: want ErrNotFound, got %v", err)
		}
	})

	t.Run("Upsert", func(t *testing.T) {
		ctx := context.Background()
		if err := s.Settings.Set(ctx, "lang", "en"); err != nil {
			t.Fatalf("Set initial: %v", err)
		}
		if err := s.Settings.Set(ctx, "lang", "fr"); err != nil {
			t.Fatalf("Set overwrite: %v", err)
		}
		got, err := s.Settings.Get(ctx, "lang")
		if err != nil {
			t.Fatalf("Get after upsert: %v", err)
		}
		if got != "fr" {
			t.Errorf("after upsert Get = %q, want fr", got)
		}
	})
}
