// Package settingtest is the shared contract test suite for the
// ports.SettingsRepository port (see ADR 0003's contract-test convention).
// It is a normal buildable package, not a _test.go file, because Go test
// files cannot be imported across packages — every adapter
// (internal/adapters/store/setting today, others later) imports this from
// its own test file and runs it against its own constructor, proving
// Liskov substitutability without duplicating the assertions per adapter.
package settingtest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty SettingsRepository for the
// duration of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.SettingsRepository

// TestSettingsRepository runs the shared SettingsRepository contract
// against newRepo. Each check is its own top-level subtest so a single
// failure identifies exactly which part of the contract broke.
func TestSettingsRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the setting", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with a duplicate key returns ErrConflict", func(t *testing.T) { testCreateDuplicate(t, newRepo) })
	t.Run("update replaces an existing setting", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing setting returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("delete removes a setting", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing setting returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("list returns every created setting across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.SettingsRepository, s *domain.Setting) {
	t.Helper()
	if err := r.Create(context.Background(), s); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), "missing.key")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	s := sampleSetting("pipeline.confidence_threshold")
	mustCreate(t, r, s)

	got, err := r.Get(context.Background(), s.Key)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != s.Value {
		t.Fatalf("Get returned Value %q, want %q", got.Value, s.Value)
	}
}

func testCreateDuplicate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleSetting("pipeline.confidence_threshold"))

	err := r.Create(context.Background(), sampleSetting("pipeline.confidence_threshold"))
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with duplicate key returned %v, want ErrConflict", err)
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	s := sampleSetting("pipeline.confidence_threshold")
	mustCreate(t, r, s)

	s.Value = `0.9`
	if err := r.Update(context.Background(), s); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), s.Key)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != `0.9` {
		t.Fatalf("Get after Update returned Value %q, want %q", got.Value, `0.9`)
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleSetting("missing.key"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing setting returned %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleSetting("pipeline.confidence_threshold"))

	if err := r.Delete(context.Background(), "pipeline.confidence_threshold"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, err := r.Get(context.Background(), "pipeline.confidence_threshold")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing.key")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing setting returned %v, want ErrNotFound", err)
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		key := fmt.Sprintf("module.test%d.enabled", i)
		mustCreate(t, r, sampleSetting(key))
		want[key] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		settings, next, err := r.List(ctx, 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, s := range settings {
			got[s.Key] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d settings, want %d", len(got), len(want))
	}
	for key := range want {
		if !got[key] {
			t.Errorf("List across pages missing setting %q", key)
		}
	}
}

func sampleSetting(key string) *domain.Setting {
	return &domain.Setting{
		Key:   key,
		Value: `true`,
	}
}
