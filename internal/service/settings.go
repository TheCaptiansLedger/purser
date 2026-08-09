package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
)

// MaskedSecretValue is the fixed placeholder GetSettings/UpdateSettings/
// ResetSetting return for a secret key's value instead of the real
// setting — see docs/adr/0028-layered-settings.md's "Secret masking"
// section. A secret key that's unset returns an empty string instead, so
// the two states ("never configured" vs. "configured but write-only")
// stay distinguishable without ever exposing the real value.
const MaskedSecretValue = "********"

// SettingSource identifies which layer produced a config key's effective
// value — SettingsService's own copy of config.Source's shape, so this
// package stays free of any internal/config dependency (the composition
// root converts config.Source into a SettingSource when adapting
// *config.Live to LiveConfig), consistent with every other service (see
// scan.go's RootContentType for the same pattern).
type SettingSource string

// The complete set of valid SettingSource values, mirroring config.Source.
const (
	SettingSourceDefault SettingSource = "default"
	SettingSourceEnv     SettingSource = "env"
	SettingSourceYAML    SettingSource = "yaml"
	SettingSourceDB      SettingSource = "db"
)

// SettingLockReason identifies why a key is rejected by
// UpdateSettings/ResetSetting — SettingsService's own copy of
// config.LockReason's shape, for the same reason SettingSource mirrors
// config.Source rather than importing it.
type SettingLockReason string

// The complete set of valid SettingLockReason values, mirroring
// config.LockReason. SettingLockReasonNone means the key isn't locked.
const (
	SettingLockReasonNone      SettingLockReason = ""
	SettingLockReasonBootstrap SettingLockReason = "bootstrap"
	SettingLockReasonOperator  SettingLockReason = "operator"
)

// KeyStatus is SettingsService's own copy of config.KeyStatus's shape: one
// config key's resolved value, source, and lock state. LiveConfig returns
// this rather than config.KeyStatus directly so internal/service never
// imports internal/config — the composition root's adapter around
// *config.Live does the field-by-field conversion.
type KeyStatus struct {
	Key    string
	Value  any
	Source SettingSource
	Secret bool

	Locked     bool
	LockReason SettingLockReason
}

// LiveConfig is the narrow interface SettingsService depends on for the
// DB-overlay layer's resolved state: Statuses for the per-key
// value/source/lock snapshot, Refresh to re-run the merge pass after a
// write. The real implementation wraps *config.Live in cmd/purser's
// composition root — see this file's package doc comment on KeyStatus for
// why the interface speaks KeyStatus, not config.KeyStatus. Declared as an
// interface, not depended on concretely, so a test can fake it without a
// real Viper/config.Load pass — the same DIP reasoning
// docs/adr/0002-solid-design-principles.md applies to every ports
// interface, adapted for a capability that isn't itself a swappable
// backend (there's only one real implementation, in-process, unlike a
// port).
type LiveConfig interface {
	Statuses() []KeyStatus
	Refresh(ctx context.Context) error
}

// SettingView is SettingsService's own read shape for one config key —
// value/source/lock metadata plus secret-aware masking already applied,
// with zero proto/Connect knowledge per docs/adr/0011-api-design.md. The
// Connect handler translates this into the wire Setting message.
type SettingView struct {
	Key        string
	Value      string
	Source     SettingSource
	Secret     bool
	Locked     bool
	LockReason SettingLockReason
}

// SettingsService orchestrates the DB-overlay layer docs/adr/0028-layered-settings.md
// defines: liveConfig for the resolved value/source/lock snapshot,
// ports.SettingsRepository for persisting/clearing the DB-stored override
// itself. It touches no other entity's port, per
// docs/adr/0011-api-design.md's SRP-per-entity rule.
type SettingsService struct {
	live LiveConfig
	repo ports.SettingsRepository
}

// NewSettingsService constructs a SettingsService backed by live and repo.
func NewSettingsService(live LiveConfig, repo ports.SettingsRepository) *SettingsService {
	return &SettingsService{live: live, repo: repo}
}

// GetSettings returns every config key's current value/source/lock
// state, in no particular order — the full key set internal/config's
// schema defines, including bootstrap-locked keys (see
// docs/adr/0028-layered-settings.md).
func (s *SettingsService) GetSettings(_ context.Context) ([]SettingView, error) {
	statuses := s.live.Statuses()
	views := make([]SettingView, 0, len(statuses))
	for _, st := range statuses {
		view, err := settingViewOf(st)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// UpdateSettings writes the JSON-encoded values named in values for every
// key in keys — the field-mask paths docs/adr/0011-api-design.md's
// FieldMask convention requires, adapted to Setting's keyed-bag shape (see
// the UpdateSettingsRequest proto doc comment). Every key must be a known,
// unlocked config key and every value must be well-formed JSON before
// anything is written: locks are checked for the whole batch first, so a
// request that touches one locked key writes nothing, not a partial batch
// (docs/adr/0028-layered-settings.md's self-audit item 2). Returns the
// post-write SettingView for every key in keys, in that order.
func (s *SettingsService) UpdateSettings(ctx context.Context, values map[string]string, keys []string) ([]SettingView, error) {
	if err := validateUpdateMask(values, keys); err != nil {
		return nil, err
	}

	statusByKey := indexStatuses(s.live.Statuses())
	for _, key := range keys {
		st, ok := statusByKey[key]
		if !ok {
			return nil, fmt.Errorf("%w: unknown setting key %q", ports.ErrNotFound, key)
		}
		if st.Locked {
			return nil, fmt.Errorf("%w: %s", ports.ErrLocked, key)
		}
		if !json.Valid([]byte(values[key])) {
			return nil, &domain.ValidationError{Errors: []domain.FieldError{
				{Field: key, Rule: "json", Value: values[key]},
			}}
		}
	}

	for _, key := range keys {
		if err := s.upsert(ctx, key, values[key]); err != nil {
			return nil, err
		}
	}

	if err := s.live.Refresh(ctx); err != nil {
		return nil, err
	}

	return s.viewsFor(keys)
}

// ResetSetting clears key's DB-stored override, so it falls back to
// yaml/env/default per the normal precedence chain. Rejected with
// ports.ErrLocked if key is locked, the same as UpdateSettings. Resetting
// a key with no stored override is a no-op, not an error — the end state
// ("no DB override for this key") is already what was asked for.
func (s *SettingsService) ResetSetting(ctx context.Context, key string) (SettingView, error) {
	statusByKey := indexStatuses(s.live.Statuses())
	st, ok := statusByKey[key]
	if !ok {
		return SettingView{}, fmt.Errorf("%w: unknown setting key %q", ports.ErrNotFound, key)
	}
	if st.Locked {
		return SettingView{}, fmt.Errorf("%w: %s", ports.ErrLocked, key)
	}

	if err := s.repo.Delete(ctx, key); err != nil && !errors.Is(err, ports.ErrNotFound) {
		return SettingView{}, err
	}

	if err := s.live.Refresh(ctx); err != nil {
		return SettingView{}, err
	}

	views, err := s.viewsFor([]string{key})
	if err != nil {
		return SettingView{}, err
	}
	return views[0], nil
}

// upsert writes value under key via ports.SettingsRepository, which has no
// upsert primitive of its own (docs/adr/0012-datastore-persistence.md's
// plain Create/Get/Update/Delete shape) — Get first to decide whether this
// is the key's first stored override or a replacement of an existing one.
func (s *SettingsService) upsert(ctx context.Context, key, value string) error {
	setting := &domain.Setting{Key: key, Value: value}
	if err := setting.Validate(); err != nil {
		return err
	}

	_, err := s.repo.Get(ctx, key)
	switch {
	case err == nil:
		return s.repo.Update(ctx, setting)
	case errors.Is(err, ports.ErrNotFound):
		return s.repo.Create(ctx, setting)
	default:
		return err
	}
}

// viewsFor returns the current SettingView for every key, in order,
// reading from the live snapshot's most recent Statuses().
func (s *SettingsService) viewsFor(keys []string) ([]SettingView, error) {
	statusByKey := indexStatuses(s.live.Statuses())
	views := make([]SettingView, 0, len(keys))
	for _, key := range keys {
		st, ok := statusByKey[key]
		if !ok {
			return nil, fmt.Errorf("%w: unknown setting key %q", ports.ErrNotFound, key)
		}
		view, err := settingViewOf(st)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func indexStatuses(statuses []KeyStatus) map[string]KeyStatus {
	byKey := make(map[string]KeyStatus, len(statuses))
	for _, st := range statuses {
		byKey[st.Key] = st
	}
	return byKey
}

// settingViewOf converts one KeyStatus into its SettingView, JSON-encoding
// Value and applying secret masking per
// docs/adr/0028-layered-settings.md.
func settingViewOf(st KeyStatus) (SettingView, error) {
	var value string
	if st.Secret {
		if s, _ := st.Value.(string); s != "" {
			value = MaskedSecretValue
		}
	} else {
		raw, err := json.Marshal(st.Value)
		if err != nil {
			return SettingView{}, fmt.Errorf("service: encoding value for setting %q: %w", st.Key, err)
		}
		value = string(raw)
	}

	return SettingView{
		Key:        st.Key,
		Value:      value,
		Source:     st.Source,
		Secret:     st.Secret,
		Locked:     st.Locked,
		LockReason: st.LockReason,
	}, nil
}

// validateUpdateMask checks that values and keys name exactly the same set
// of config keys — the "a key present in update_mask.paths but absent
// from values (or vice versa) is a validation error" rule the
// UpdateSettingsRequest proto doc comment states.
func validateUpdateMask(values map[string]string, keys []string) error {
	if len(keys) == 0 {
		return &domain.ValidationError{Errors: []domain.FieldError{
			{Field: "update_mask", Rule: "required", Value: ""},
		}}
	}
	if len(keys) != len(values) {
		return &domain.ValidationError{Errors: []domain.FieldError{
			{Field: "update_mask", Rule: "matches_values", Value: fmt.Sprintf("%d paths, %d values", len(keys), len(values))},
		}}
	}
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			return &domain.ValidationError{Errors: []domain.FieldError{
				{Field: "values", Rule: "required", Value: key},
			}}
		}
	}
	return nil
}
