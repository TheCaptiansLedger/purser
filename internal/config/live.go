package config

import (
	"context"
	"fmt"
	"purser/internal/ports"
	"sync/atomic"

	"github.com/spf13/viper"
)

// snapshot bundles one ApplyOverrides result — a fully-resolved Config
// alongside the per-key provenance computed in the same pass — behind a
// single pointer, so Live's atomic swap never lets a caller observe a
// Config from one merge pass paired with KeyStatus from another.
type snapshot struct {
	cfg      Config
	statuses []KeyStatus
}

// Live holds the DB-overlay-eligible subset of Config as an atomically
// swapped snapshot of the ApplyOverrides result — see
// docs/adr/0028-layered-settings.md's "Runtime effect without restart"
// section. Get and Statuses are each a single atomic load, safe for
// concurrent use by any number of goroutines: a service holding a *Live
// reference observes the result of the most recent successful Refresh
// without a restart, and never a partially applied merge pass.
//
// Live never touches the bootstrap-locked subset (database.*) —
// ApplyOverrides itself excludes those keys from the DB-overlay entirely.
// Callers needing bootstrap config keep using the one-shot Config Load
// returns directly, exactly as before; Live only matters for the fields
// ADR 0028 calls "DB-overlay-eligible."
type Live struct {
	configPath string
	repo       ports.SettingsRepository

	current atomic.Pointer[snapshot]
}

// NewLive runs the merge pass once against configPath/repo and returns a
// *Live seeded with that result, or an error if the initial pass fails —
// the same fail-fast-at-startup posture Config.Validate already uses.
func NewLive(ctx context.Context, configPath string, repo ports.SettingsRepository) (*Live, error) {
	l := &Live{configPath: configPath, repo: repo}
	if err := l.Refresh(ctx); err != nil {
		return nil, err
	}
	return l, nil
}

// Get returns the most recently applied Config snapshot. Safe for
// concurrent use.
func (l *Live) Get() Config {
	return l.current.Load().cfg
}

// Statuses returns the most recently computed per-key source/lock
// provenance, for a future SettingsService's GetSettings. Safe for
// concurrent use.
func (l *Live) Statuses() []KeyStatus {
	return l.current.Load().statuses
}

// Refresh re-runs the merge pass and, only on success, atomically swaps
// the new Config/KeyStatus pair in as the current snapshot. On error the
// previous snapshot is left in place unchanged — a bad write (or a
// repository error) must never leave Get returning a half-applied or
// invalid Config. A future SettingsService calls this once immediately
// after a successful UpdateSettings/ResetSetting write
// (docs/adr/0028-layered-settings.md's "Runtime effect without restart").
//
// Refresh builds a fresh *viper.Viper and re-runs Load on every call
// rather than reusing one long-lived instance across the process
// lifetime. This matters specifically for ResetSetting: viper.Set (which
// ApplyOverrides uses to layer in a DB value) is Viper's own
// highest-precedence write and has no corresponding "unset" — reusing one
// mutable Viper instance across repeated Refresh calls would make a
// deleted Setting's last-known value stick forever instead of falling
// back to default. Starting clean each time trades a cheap re-read of
// defaults/env/yaml (no extra I/O beyond re-parsing configPath, which
// already happens once at startup) for every Refresh being idempotent
// regardless of write/reset history.
func (l *Live) Refresh(ctx context.Context) error {
	v := viper.New()
	if _, err := Load(v, l.configPath); err != nil {
		return fmt.Errorf("config: refreshing live snapshot: %w", err)
	}

	cfg, statuses, err := ApplyOverrides(ctx, v, l.configPath, l.repo)
	if err != nil {
		return err
	}
	l.current.Store(&snapshot{cfg: cfg, statuses: statuses})
	return nil
}
