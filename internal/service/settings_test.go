package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeLiveConfig struct {
	statuses          []service.KeyStatus
	refreshedStatuses []service.KeyStatus
	refreshErr        error
	refreshCalls      int
}

func (f *fakeLiveConfig) Statuses() []service.KeyStatus { return f.statuses }

func (f *fakeLiveConfig) Refresh(_ context.Context) error {
	f.refreshCalls++
	if f.refreshErr != nil {
		return f.refreshErr
	}
	if f.refreshedStatuses != nil {
		f.statuses = f.refreshedStatuses
	}
	return nil
}

type fakeSettingsRepository struct {
	data map[string]string

	getErr    error
	createErr error
	updateErr error
	deleteErr error

	createCalls []string
	updateCalls []string
	deleteCalls []string
}

func newFakeSettingsRepository(initial map[string]string) *fakeSettingsRepository {
	if initial == nil {
		initial = map[string]string{}
	}
	return &fakeSettingsRepository{data: initial}
}

func (f *fakeSettingsRepository) Create(_ context.Context, s *domain.Setting) error {
	f.createCalls = append(f.createCalls, s.Key)
	if f.createErr != nil {
		return f.createErr
	}
	f.data[s.Key] = s.Value
	return nil
}

func (f *fakeSettingsRepository) Get(_ context.Context, key string) (*domain.Setting, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	v, ok := f.data[key]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return &domain.Setting{Key: key, Value: v}, nil
}

func (f *fakeSettingsRepository) Update(_ context.Context, s *domain.Setting) error {
	f.updateCalls = append(f.updateCalls, s.Key)
	if f.updateErr != nil {
		return f.updateErr
	}
	f.data[s.Key] = s.Value
	return nil
}

func (f *fakeSettingsRepository) Delete(_ context.Context, key string) error {
	f.deleteCalls = append(f.deleteCalls, key)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.data[key]; !ok {
		return ports.ErrNotFound
	}
	delete(f.data, key)
	return nil
}

func (f *fakeSettingsRepository) List(_ context.Context, _ int, _ string) ([]*domain.Setting, string, error) {
	return nil, "", nil
}

func TestSettingsService_GetSettings(t *testing.T) {
	live := &fakeLiveConfig{statuses: []service.KeyStatus{
		{Key: "pipeline.confidence_threshold", Value: 0.75, Source: service.SettingSourceDefault},
		{Key: "sources.stashdb.api_key", Value: "real-key", Source: service.SettingSourceDB, Secret: true},
		{Key: "sources.tpdb.api_key", Value: "", Source: service.SettingSourceDefault, Secret: true},
		{Key: "database.driver", Value: "badger", Source: service.SettingSourceDefault, Locked: true, LockReason: service.SettingLockReasonBootstrap},
	}}
	svc := service.NewSettingsService(live, newFakeSettingsRepository(nil))

	got, err := svc.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings returned error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("GetSettings returned %d views, want 4", len(got))
	}

	byKey := map[string]service.SettingView{}
	for _, v := range got {
		byKey[v.Key] = v
	}

	if v := byKey["pipeline.confidence_threshold"]; v.Value != "0.75" {
		t.Fatalf("non-secret Value = %q, want JSON-encoded 0.75", v.Value)
	}
	if v := byKey["sources.stashdb.api_key"]; v.Value != service.MaskedSecretValue {
		t.Fatalf("set secret Value = %q, want masked placeholder", v.Value)
	}
	if v := byKey["sources.tpdb.api_key"]; v.Value != "" {
		t.Fatalf("unset secret Value = %q, want empty", v.Value)
	}
	if v := byKey["database.driver"]; !v.Locked || v.LockReason != service.SettingLockReasonBootstrap {
		t.Fatalf("bootstrap key view = %+v, want Locked=true LockReason=bootstrap", v)
	}
}

func TestSettingsService_UpdateSettings(t *testing.T) {
	live := &fakeLiveConfig{
		statuses: []service.KeyStatus{
			{Key: "pipeline.confidence_threshold", Value: 0.75, Source: service.SettingSourceDefault},
		},
		refreshedStatuses: []service.KeyStatus{
			{Key: "pipeline.confidence_threshold", Value: 0.9, Source: service.SettingSourceDB},
		},
	}
	repo := newFakeSettingsRepository(nil)
	svc := service.NewSettingsService(live, repo)

	got, err := svc.UpdateSettings(context.Background(),
		map[string]string{"pipeline.confidence_threshold": "0.9"},
		[]string{"pipeline.confidence_threshold"})
	if err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}
	if len(got) != 1 || got[0].Value != "0.9" || got[0].Source != service.SettingSourceDB {
		t.Fatalf("UpdateSettings returned %+v, want a single refreshed 0.9/db view", got)
	}
	if len(repo.createCalls) != 1 || repo.createCalls[0] != "pipeline.confidence_threshold" {
		t.Fatalf("UpdateSettings createCalls = %v, want [pipeline.confidence_threshold] (key didn't exist yet)", repo.createCalls)
	}
	if live.refreshCalls != 1 {
		t.Fatalf("UpdateSettings called Refresh %d times, want 1", live.refreshCalls)
	}
}

func TestSettingsService_UpdateSettings_ExistingKeyUpdates(t *testing.T) {
	live := &fakeLiveConfig{statuses: []service.KeyStatus{
		{Key: "pipeline.confidence_threshold", Value: 0.9, Source: service.SettingSourceDB},
	}}
	repo := newFakeSettingsRepository(map[string]string{"pipeline.confidence_threshold": "0.9"})
	svc := service.NewSettingsService(live, repo)

	_, err := svc.UpdateSettings(context.Background(),
		map[string]string{"pipeline.confidence_threshold": "0.5"},
		[]string{"pipeline.confidence_threshold"})
	if err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}
	if len(repo.updateCalls) != 1 {
		t.Fatalf("UpdateSettings updateCalls = %v, want one call (key already existed)", repo.updateCalls)
	}
	if len(repo.createCalls) != 0 {
		t.Fatalf("UpdateSettings createCalls = %v, want none", repo.createCalls)
	}
}

func TestSettingsService_UpdateSettings_MaskMismatch(t *testing.T) {
	live := &fakeLiveConfig{}
	repo := newFakeSettingsRepository(nil)
	svc := service.NewSettingsService(live, repo)

	_, err := svc.UpdateSettings(context.Background(),
		map[string]string{"a": "1", "b": "2"},
		[]string{"a"})

	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("UpdateSettings returned %v, want *domain.ValidationError", err)
	}
	if len(repo.createCalls)+len(repo.updateCalls) != 0 {
		t.Fatal("UpdateSettings wrote a setting despite a mask/values mismatch")
	}
}

func TestSettingsService_UpdateSettings_UnknownKey(t *testing.T) {
	live := &fakeLiveConfig{}
	svc := service.NewSettingsService(live, newFakeSettingsRepository(nil))

	_, err := svc.UpdateSettings(context.Background(),
		map[string]string{"no.such.key": `"x"`},
		[]string{"no.such.key"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("UpdateSettings returned %v, want ports.ErrNotFound", err)
	}
}

func TestSettingsService_UpdateSettings_LockedKeyBlocksWholeBatch(t *testing.T) {
	live := &fakeLiveConfig{statuses: []service.KeyStatus{
		{Key: "pipeline.confidence_threshold", Value: 0.75, Source: service.SettingSourceDefault},
		{Key: "database.driver", Value: "badger", Locked: true, LockReason: service.SettingLockReasonBootstrap},
	}}
	repo := newFakeSettingsRepository(nil)
	svc := service.NewSettingsService(live, repo)

	_, err := svc.UpdateSettings(context.Background(),
		map[string]string{"pipeline.confidence_threshold": "0.9", "database.driver": `"postgres"`},
		[]string{"pipeline.confidence_threshold", "database.driver"})
	if !errors.Is(err, ports.ErrLocked) {
		t.Fatalf("UpdateSettings returned %v, want ports.ErrLocked", err)
	}
	if len(repo.createCalls)+len(repo.updateCalls) != 0 {
		t.Fatalf("UpdateSettings wrote to the unlocked key before rejecting the locked one: create=%v update=%v", repo.createCalls, repo.updateCalls)
	}
	if live.refreshCalls != 0 {
		t.Fatal("UpdateSettings refreshed Live despite rejecting the batch")
	}
}

func TestSettingsService_UpdateSettings_InvalidJSON(t *testing.T) {
	live := &fakeLiveConfig{statuses: []service.KeyStatus{
		{Key: "pipeline.confidence_threshold", Value: 0.75, Source: service.SettingSourceDefault},
	}}
	repo := newFakeSettingsRepository(nil)
	svc := service.NewSettingsService(live, repo)

	_, err := svc.UpdateSettings(context.Background(),
		map[string]string{"pipeline.confidence_threshold": "not json"},
		[]string{"pipeline.confidence_threshold"})

	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("UpdateSettings returned %v, want *domain.ValidationError", err)
	}
	if len(repo.createCalls)+len(repo.updateCalls) != 0 {
		t.Fatal("UpdateSettings wrote a malformed value")
	}
}

func TestSettingsService_UpdateSettings_RefreshErrorPropagates(t *testing.T) {
	wantErr := errors.New("boom")
	live := &fakeLiveConfig{
		statuses:   []service.KeyStatus{{Key: "pipeline.confidence_threshold", Value: 0.75, Source: service.SettingSourceDefault}},
		refreshErr: wantErr,
	}
	svc := service.NewSettingsService(live, newFakeSettingsRepository(nil))

	_, err := svc.UpdateSettings(context.Background(),
		map[string]string{"pipeline.confidence_threshold": "0.9"},
		[]string{"pipeline.confidence_threshold"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("UpdateSettings returned %v, want %v", err, wantErr)
	}
}

func TestSettingsService_ResetSetting(t *testing.T) {
	live := &fakeLiveConfig{
		statuses:          []service.KeyStatus{{Key: "pipeline.confidence_threshold", Value: 0.9, Source: service.SettingSourceDB}},
		refreshedStatuses: []service.KeyStatus{{Key: "pipeline.confidence_threshold", Value: 0.75, Source: service.SettingSourceDefault}},
	}
	repo := newFakeSettingsRepository(map[string]string{"pipeline.confidence_threshold": "0.9"})
	svc := service.NewSettingsService(live, repo)

	got, err := svc.ResetSetting(context.Background(), "pipeline.confidence_threshold")
	if err != nil {
		t.Fatalf("ResetSetting returned error: %v", err)
	}
	if got.Value != "0.75" || got.Source != service.SettingSourceDefault {
		t.Fatalf("ResetSetting returned %+v, want the post-refresh default view", got)
	}
	if len(repo.deleteCalls) != 1 {
		t.Fatalf("ResetSetting deleteCalls = %v, want one call", repo.deleteCalls)
	}
	if live.refreshCalls != 1 {
		t.Fatalf("ResetSetting called Refresh %d times, want 1", live.refreshCalls)
	}
}

func TestSettingsService_ResetSetting_NoStoredOverrideIsNotAnError(t *testing.T) {
	live := &fakeLiveConfig{statuses: []service.KeyStatus{
		{Key: "pipeline.confidence_threshold", Value: 0.75, Source: service.SettingSourceDefault},
	}}
	repo := newFakeSettingsRepository(nil) // nothing stored for this key
	svc := service.NewSettingsService(live, repo)

	if _, err := svc.ResetSetting(context.Background(), "pipeline.confidence_threshold"); err != nil {
		t.Fatalf("ResetSetting on a key with no stored override returned error: %v", err)
	}
}

func TestSettingsService_ResetSetting_UnknownKey(t *testing.T) {
	svc := service.NewSettingsService(&fakeLiveConfig{}, newFakeSettingsRepository(nil))

	_, err := svc.ResetSetting(context.Background(), "no.such.key")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("ResetSetting returned %v, want ports.ErrNotFound", err)
	}
}

func TestSettingsService_ResetSetting_LockedKey(t *testing.T) {
	live := &fakeLiveConfig{statuses: []service.KeyStatus{
		{Key: "database.driver", Value: "badger", Locked: true, LockReason: service.SettingLockReasonBootstrap},
	}}
	repo := newFakeSettingsRepository(nil)
	svc := service.NewSettingsService(live, repo)

	_, err := svc.ResetSetting(context.Background(), "database.driver")
	if !errors.Is(err, ports.ErrLocked) {
		t.Fatalf("ResetSetting returned %v, want ports.ErrLocked", err)
	}
	if len(repo.deleteCalls) != 0 {
		t.Fatal("ResetSetting deleted a locked key")
	}
}

func TestSettingsService_ResetSetting_DeleteErrorPropagates(t *testing.T) {
	wantErr := errors.New("boom")
	live := &fakeLiveConfig{statuses: []service.KeyStatus{
		{Key: "pipeline.confidence_threshold", Value: 0.9, Source: service.SettingSourceDB},
	}}
	repo := newFakeSettingsRepository(map[string]string{"pipeline.confidence_threshold": "0.9"})
	repo.deleteErr = wantErr
	svc := service.NewSettingsService(live, repo)

	if _, err := svc.ResetSetting(context.Background(), "pipeline.confidence_threshold"); !errors.Is(err, wantErr) {
		t.Fatalf("ResetSetting returned %v, want %v", err, wantErr)
	}
}
