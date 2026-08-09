package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"

	"connectrpc.com/connect"

	settingsv1 "purser/gen/go/purser/settings/v1"

	apiconnect "purser/internal/api/connect"
)

type fakeSettingsService struct {
	getViews []service.SettingView
	getErr   error

	updateValues map[string]string
	updateKeys   []string
	updateViews  []service.SettingView
	updateErr    error

	resetKey  string
	resetView service.SettingView
	resetErr  error
}

func (f *fakeSettingsService) GetSettings(_ context.Context) ([]service.SettingView, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getViews, nil
}

func (f *fakeSettingsService) UpdateSettings(_ context.Context, values map[string]string, keys []string) ([]service.SettingView, error) {
	f.updateValues = values
	f.updateKeys = keys
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.updateViews, nil
}

func (f *fakeSettingsService) ResetSetting(_ context.Context, key string) (service.SettingView, error) {
	f.resetKey = key
	if f.resetErr != nil {
		return service.SettingView{}, f.resetErr
	}
	return f.resetView, nil
}

func TestSettingsHandler_GetSettings(t *testing.T) {
	svc := &fakeSettingsService{getViews: []service.SettingView{
		{Key: "pipeline.confidence_threshold", Value: "0.75", Source: service.SettingSourceDefault},
		{Key: "sources.stashdb.api_key", Value: service.MaskedSecretValue, Source: service.SettingSourceDB, Secret: true},
		{Key: "database.driver", Value: `"badger"`, Locked: true, LockReason: service.SettingLockReasonBootstrap},
	}}
	h := apiconnect.NewSettingsHandler(svc, nil)

	resp, err := h.GetSettings(context.Background(), connect.NewRequest(&settingsv1.GetSettingsRequest{}))
	if err != nil {
		t.Fatalf("GetSettings returned error: %v", err)
	}
	settings := resp.Msg.GetSettings()
	if len(settings) != 3 {
		t.Fatalf("GetSettings returned %d settings, want 3", len(settings))
	}

	byKey := map[string]*settingsv1.Setting{}
	for _, s := range settings {
		byKey[s.GetKey()] = s
	}

	if s := byKey["sources.stashdb.api_key"]; s.GetValue() != service.MaskedSecretValue || !s.GetSecret() || s.GetSource() != settingsv1.SettingSource_SETTING_SOURCE_DB {
		t.Fatalf("secret setting = %+v, want masked value/secret=true/source=DB", s)
	}
	if s := byKey["database.driver"]; !s.GetLocked() || s.GetLockReason() != settingsv1.SettingLockReason_SETTING_LOCK_REASON_BOOTSTRAP {
		t.Fatalf("bootstrap setting = %+v, want locked=true lock_reason=BOOTSTRAP", s)
	}
}

func TestSettingsHandler_GetSettings_Error(t *testing.T) {
	svc := &fakeSettingsService{getErr: errors.New("boom")}
	h := apiconnect.NewSettingsHandler(svc, nil)

	_, err := h.GetSettings(context.Background(), connect.NewRequest(&settingsv1.GetSettingsRequest{}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("GetSettings returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("GetSettings returned code %v, want %v", connErr.Code(), connect.CodeInternal)
	}
}

func TestSettingsHandler_UpdateSettings(t *testing.T) {
	svc := &fakeSettingsService{updateViews: []service.SettingView{
		{Key: "pipeline.confidence_threshold", Value: "0.9", Source: service.SettingSourceDB},
	}}
	h := apiconnect.NewSettingsHandler(svc, nil)

	resp, err := h.UpdateSettings(context.Background(), connect.NewRequest(&settingsv1.UpdateSettingsRequest{
		Values:     map[string]string{"pipeline.confidence_threshold": "0.9"},
		UpdateMask: []string{"pipeline.confidence_threshold"},
	}))
	if err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}
	if len(resp.Msg.GetSettings()) != 1 || resp.Msg.GetSettings()[0].GetValue() != "0.9" {
		t.Fatalf("UpdateSettings returned unexpected settings: %+v", resp.Msg.GetSettings())
	}
	if svc.updateKeys[0] != "pipeline.confidence_threshold" || svc.updateValues["pipeline.confidence_threshold"] != "0.9" {
		t.Fatalf("UpdateSettings passed keys=%v values=%v, want the request's mask/values", svc.updateKeys, svc.updateValues)
	}
}

func TestSettingsHandler_UpdateSettings_Locked(t *testing.T) {
	svc := &fakeSettingsService{updateErr: ports.ErrLocked}
	h := apiconnect.NewSettingsHandler(svc, nil)

	_, err := h.UpdateSettings(context.Background(), connect.NewRequest(&settingsv1.UpdateSettingsRequest{
		Values:     map[string]string{"database.driver": `"postgres"`},
		UpdateMask: []string{"database.driver"},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("UpdateSettings returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("UpdateSettings returned code %v, want %v", connErr.Code(), connect.CodeFailedPrecondition)
	}
}

func TestSettingsHandler_ResetSetting(t *testing.T) {
	svc := &fakeSettingsService{resetView: service.SettingView{
		Key: "pipeline.confidence_threshold", Value: "0.75", Source: service.SettingSourceDefault,
	}}
	h := apiconnect.NewSettingsHandler(svc, nil)

	resp, err := h.ResetSetting(context.Background(), connect.NewRequest(&settingsv1.ResetSettingRequest{Key: "pipeline.confidence_threshold"}))
	if err != nil {
		t.Fatalf("ResetSetting returned error: %v", err)
	}
	if resp.Msg.GetSetting().GetValue() != "0.75" {
		t.Fatalf("ResetSetting returned %+v, want value 0.75", resp.Msg.GetSetting())
	}
	if svc.resetKey != "pipeline.confidence_threshold" {
		t.Fatalf("ResetSetting passed key %q, want %q", svc.resetKey, "pipeline.confidence_threshold")
	}
}

func TestSettingsHandler_ResetSetting_NotFound(t *testing.T) {
	svc := &fakeSettingsService{resetErr: ports.ErrNotFound}
	h := apiconnect.NewSettingsHandler(svc, nil)

	_, err := h.ResetSetting(context.Background(), connect.NewRequest(&settingsv1.ResetSettingRequest{Key: "no.such.key"}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("ResetSetting returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("ResetSetting returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}
