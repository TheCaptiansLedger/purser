package config

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/ports"
	"strings"

	"github.com/spf13/viper"
)

// Service implements ports.ConfigService by merging operator config (env/file)
// with DB-stored settings, with built-in Viper defaults as final fallback.
type Service struct {
	v        *viper.Viper
	locked   map[string]struct{}
	settings ports.SettingsRepository
}

// New returns a ConfigService. v and locked must come from config.LoadFull —
// v provides default-value fallback, locked carries the operator-managed key set.
func New(v *viper.Viper, locked map[string]struct{}, settings ports.SettingsRepository) *Service {
	return &Service{v: v, locked: locked, settings: settings}
}

// IsLocked reports whether key is managed by the operator via env var or YAML
// file and therefore cannot be overwritten from the UI.
func (s *Service) IsLocked(key string) bool {
	_, ok := s.locked[strings.ToLower(key)]
	return ok
}

// Get returns the effective value for key using the precedence chain:
// operator (env/file) → DB → built-in default.
func (s *Service) Get(ctx context.Context, key string) (string, error) {
	if s.IsLocked(key) {
		return s.v.GetString(key), nil
	}
	val, err := s.settings.Get(ctx, key)
	if err == nil {
		return val, nil
	}
	if !errors.Is(err, errs.ErrNotFound) {
		return "", fmt.Errorf("get config %s: %w", key, err)
	}
	return s.v.GetString(key), nil
}

// Set writes a runtime config value to the DB.
// Returns errs.ErrLocked if the key is currently managed by the operator.
func (s *Service) Set(ctx context.Context, key, value string) error {
	if s.IsLocked(key) {
		return errs.ErrLocked
	}
	return s.settings.Set(ctx, key, value)
}

var _ ports.ConfigService = (*Service)(nil)
