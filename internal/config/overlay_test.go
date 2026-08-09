package config_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"

	"github.com/spf13/viper"
)

// fakeSettingsRepository is a minimal in-memory ports.SettingsRepository
// for testing ApplyOverrides — it doesn't honor pageSize (returns
// everything on the first page), which is fine for a hand-rolled fake
// exercising a consumer, not the port's own contract (see
// internal/ports/settingtest for that).
type fakeSettingsRepository struct {
	data    map[string]string
	listErr error
}

func newFakeSettingsRepository(initial map[string]string) *fakeSettingsRepository {
	return &fakeSettingsRepository{data: initial}
}

func (f *fakeSettingsRepository) Create(_ context.Context, s *domain.Setting) error {
	f.data[s.Key] = s.Value
	return nil
}

func (f *fakeSettingsRepository) Get(_ context.Context, key string) (*domain.Setting, error) {
	v, ok := f.data[key]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return &domain.Setting{Key: key, Value: v}, nil
}

func (f *fakeSettingsRepository) Update(_ context.Context, s *domain.Setting) error {
	f.data[s.Key] = s.Value
	return nil
}

func (f *fakeSettingsRepository) Delete(_ context.Context, key string) error {
	delete(f.data, key)
	return nil
}

func (f *fakeSettingsRepository) List(_ context.Context, _ int, _ string) ([]*domain.Setting, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	settings := make([]*domain.Setting, 0, len(f.data))
	for k, v := range f.data {
		settings = append(settings, &domain.Setting{Key: k, Value: v})
	}
	return settings, "", nil
}

func statusOf(statuses []config.KeyStatus, key string) (config.KeyStatus, bool) {
	for _, s := range statuses {
		if s.Key == key {
			return s, true
		}
	}
	return config.KeyStatus{}, false
}

func TestApplyOverrides_AppliesDBValueForUnlockedKey(t *testing.T) {
	v := viper.New()
	if _, err := config.Load(v, ""); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `0.9`,
	})

	cfg, statuses, err := config.ApplyOverrides(context.Background(), v, "", repo)
	if err != nil {
		t.Fatalf("ApplyOverrides returned error: %v", err)
	}
	if cfg.Pipeline.ConfidenceThreshold != 0.9 {
		t.Fatalf("ApplyOverrides ConfidenceThreshold = %v, want 0.9", cfg.Pipeline.ConfidenceThreshold)
	}

	st, ok := statusOf(statuses, "pipeline.confidence_threshold")
	if !ok {
		t.Fatal("statuses missing pipeline.confidence_threshold")
	}
	if st.Source != config.SourceDB || st.Locked {
		t.Fatalf("status = %+v, want Source=db Locked=false", st)
	}
}

func TestApplyOverrides_EnvLockedKeyIgnoresDBValue(t *testing.T) {
	t.Setenv("PURSER_PIPELINE_CONFIDENCE_THRESHOLD", "0.5")

	v := viper.New()
	if _, err := config.Load(v, ""); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `0.9`,
	})

	cfg, statuses, err := config.ApplyOverrides(context.Background(), v, "", repo)
	if err != nil {
		t.Fatalf("ApplyOverrides returned error: %v", err)
	}
	if cfg.Pipeline.ConfidenceThreshold != 0.5 {
		t.Fatalf("ApplyOverrides ConfidenceThreshold = %v, want 0.5 (env must win over DB)", cfg.Pipeline.ConfidenceThreshold)
	}

	st, ok := statusOf(statuses, "pipeline.confidence_threshold")
	if !ok {
		t.Fatal("statuses missing pipeline.confidence_threshold")
	}
	if !st.Locked || st.LockReason != config.LockReasonOperator || st.Source != config.SourceEnv {
		t.Fatalf("status = %+v, want Locked=true LockReason=operator Source=env", st)
	}
}

func TestApplyOverrides_YAMLLockedKeyIgnoresDBValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "purser.yaml")
	if err := os.WriteFile(path, []byte("pipeline:\n  confidence_threshold: 0.6\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	v := viper.New()
	if _, err := config.Load(v, path); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `0.9`,
	})

	cfg, statuses, err := config.ApplyOverrides(context.Background(), v, path, repo)
	if err != nil {
		t.Fatalf("ApplyOverrides returned error: %v", err)
	}
	if cfg.Pipeline.ConfidenceThreshold != 0.6 {
		t.Fatalf("ApplyOverrides ConfidenceThreshold = %v, want 0.6 (yaml must win over DB)", cfg.Pipeline.ConfidenceThreshold)
	}

	st, ok := statusOf(statuses, "pipeline.confidence_threshold")
	if !ok {
		t.Fatal("statuses missing pipeline.confidence_threshold")
	}
	if !st.Locked || st.LockReason != config.LockReasonOperator || st.Source != config.SourceYAML {
		t.Fatalf("status = %+v, want Locked=true LockReason=operator Source=yaml", st)
	}
}

func TestApplyOverrides_BootstrapKeyIgnoresDBValueEvenWhenUnset(t *testing.T) {
	v := viper.New()
	if _, err := config.Load(v, ""); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	repo := newFakeSettingsRepository(map[string]string{
		"database.driver": `"postgres"`,
	})

	cfg, statuses, err := config.ApplyOverrides(context.Background(), v, "", repo)
	if err != nil {
		t.Fatalf("ApplyOverrides returned error: %v", err)
	}
	if cfg.Database.Driver == "postgres" {
		t.Fatalf("ApplyOverrides applied a DB value to a bootstrap key (database.driver = %q)", cfg.Database.Driver)
	}

	st, ok := statusOf(statuses, "database.driver")
	if !ok {
		t.Fatal("statuses missing database.driver")
	}
	if !st.Locked || st.LockReason != config.LockReasonBootstrap {
		t.Fatalf("status = %+v, want Locked=true LockReason=bootstrap", st)
	}
}

func TestApplyOverrides_NoOverrideKeepsSourceDefault(t *testing.T) {
	v := viper.New()
	if _, err := config.Load(v, ""); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	cfg, statuses, err := config.ApplyOverrides(context.Background(), v, "", newFakeSettingsRepository(map[string]string{}))
	if err != nil {
		t.Fatalf("ApplyOverrides returned error: %v", err)
	}
	if cfg.Pipeline.ConfidenceThreshold != 0.75 {
		t.Fatalf("ApplyOverrides ConfidenceThreshold = %v, want default 0.75", cfg.Pipeline.ConfidenceThreshold)
	}

	st, ok := statusOf(statuses, "pipeline.confidence_threshold")
	if !ok {
		t.Fatal("statuses missing pipeline.confidence_threshold")
	}
	if st.Locked || st.Source != config.SourceDefault {
		t.Fatalf("status = %+v, want Locked=false Source=default", st)
	}
}

func TestApplyOverrides_PreservesDerivedBootstrapDefaults(t *testing.T) {
	v := viper.New()
	loaded, err := config.Load(v, "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	cfg, _, err := config.ApplyOverrides(context.Background(), v, "", newFakeSettingsRepository(map[string]string{}))
	if err != nil {
		t.Fatalf("ApplyOverrides returned error: %v", err)
	}
	if cfg.Database.Badger.DataDir != loaded.Database.Badger.DataDir {
		t.Fatalf("ApplyOverrides Badger.DataDir = %q, want %q (derived default must survive the overlay pass)",
			cfg.Database.Badger.DataDir, loaded.Database.Badger.DataDir)
	}
}

func TestApplyOverrides_InvalidStoredJSONReturnsError(t *testing.T) {
	v := viper.New()
	if _, err := config.Load(v, ""); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `not valid json`,
	})

	if _, _, err := config.ApplyOverrides(context.Background(), v, "", repo); err == nil {
		t.Fatal("ApplyOverrides with invalid stored JSON did not return an error")
	}
}

func TestApplyOverrides_InvalidResultingConfigReturnsError(t *testing.T) {
	v := viper.New()
	if _, err := config.Load(v, ""); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	repo := newFakeSettingsRepository(map[string]string{
		// Missing content_type makes Pipeline.Validate reject this.
		"pipeline.scan_roots": `[{"path":"/movies"}]`,
	})

	if _, _, err := config.ApplyOverrides(context.Background(), v, "", repo); err == nil {
		t.Fatal("ApplyOverrides with an invalid resulting config did not return an error")
	}
}

func TestApplyOverrides_SettingsRepositoryListErrorPropagates(t *testing.T) {
	v := viper.New()
	if _, err := config.Load(v, ""); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	repo := newFakeSettingsRepository(map[string]string{})
	repo.listErr = errors.New("datastore unavailable")

	if _, _, err := config.ApplyOverrides(context.Background(), v, "", repo); err == nil {
		t.Fatal("ApplyOverrides with a failing SettingsRepository.List did not return an error")
	}
}

func TestApplyOverrides_MissingConfigFileForLockDetectionReturnsError(t *testing.T) {
	v := viper.New()
	path := filepath.Join(t.TempDir(), "purser.yaml")
	if err := os.WriteFile(path, []byte("server:\n  listen_addr: \":7070\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := config.Load(v, path); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Simulate the file disappearing between Load and ApplyOverrides.
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	_, _, err := config.ApplyOverrides(context.Background(), v, path, newFakeSettingsRepository(map[string]string{}))
	if err == nil {
		t.Fatal("ApplyOverrides with a missing config file did not return an error")
	}
	if errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("ApplyOverrides returned ports.ErrNotFound, want a plain config-reading error")
	}
}
