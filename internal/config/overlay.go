package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"purser/internal/ports"
	"strings"

	"github.com/spf13/viper"
)

// Source identifies which layer produced a config key's effective value.
// See docs/adr/0028-layered-settings.md.
type Source string

// The complete set of valid Source values.
const (
	SourceDefault Source = "default"
	SourceEnv     Source = "env"
	SourceYAML    Source = "yaml"
	SourceDB      Source = "db"
)

// LockReason identifies why ApplyOverrides will never write a key's value
// from the DB-overlay layer. LockReasonNone means the key isn't locked at
// all. See docs/adr/0028-layered-settings.md.
type LockReason string

// The complete set of valid LockReason values.
const (
	LockReasonNone      LockReason = ""
	LockReasonBootstrap LockReason = "bootstrap"
	LockReasonOperator  LockReason = "operator"
)

// KeyStatus describes one config key's effective value, source, lock
// state, and secret classification after ApplyOverrides runs — the
// metadata SettingsService's GetSettings exposes to the UI for value
// display and provenance.
type KeyStatus struct {
	Key    string
	Value  any
	Source Source
	Secret bool

	Locked     bool
	LockReason LockReason
}

// bootstrapKeyPrefixes are the dotted key prefixes ApplyOverrides never
// reads from or writes to the DB-overlay layer — the datastore itself
// isn't reachable until these are already resolved (the chicken-and-egg
// problem docs/adr/0028-layered-settings.md names explicitly).
var bootstrapKeyPrefixes = []string{"server.", "database.", "telemetry.", "log.", "paths."}

func isBootstrapKey(key string) bool {
	for _, p := range bootstrapKeyPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// envVarName returns the PURSER_ environment variable name Load's
// SetEnvKeyReplacer maps key to.
func envVarName(key string) string {
	return "PURSER_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
}

// ApplyOverrides re-resolves v's DB-overlay-eligible keys (every key
// except the bootstrap prefixes above) against repo, applying a stored
// Setting only where the key isn't already locked by an explicit env var
// or YAML entry.
//
// v must be the same *viper.Viper instance already passed to Load for
// this process — ApplyOverrides builds on the defaults/file/env layers
// Load already registered on it rather than re-deriving them, and calls
// v.Set only for keys confirmed unlocked first (see
// docs/adr/0028-layered-settings.md: calling it unconditionally would let
// a DB value silently outrank an operator's env/yaml value, since
// viper.Set is Viper's own highest-precedence write).
//
// Cobra flag precedence (docs/adr/0010-configuration.md) is not yet a
// lock condition here: no key in this codebase is bound to a flag today
// (config.Load is always called with a fresh, unbound *viper.Viper), so
// there is nothing to detect. Binding a flag to a DB-overlay-eligible key
// in the future requires extending the lock detection below alongside
// that change — a flag-bound key must lock the same way env/yaml do, or
// ApplyOverrides's v.Set would silently clobber it.
func ApplyOverrides(ctx context.Context, v *viper.Viper, configPath string, repo ports.SettingsRepository) (Config, []KeyStatus, error) {
	keys := v.AllKeys()

	operatorSources, err := operatorLockedSources(configPath, keys)
	if err != nil {
		return Config{}, nil, err
	}

	dbValues, err := allSettingValues(ctx, repo)
	if err != nil {
		return Config{}, nil, err
	}

	statuses := make([]KeyStatus, 0, len(keys))
	for _, key := range keys {
		secret := isSecretKey(key)

		switch {
		case isBootstrapKey(key):
			statuses = append(statuses, KeyStatus{
				Key:        key,
				Value:      v.Get(key),
				Source:     sourceOrDefault(operatorSources, key),
				Secret:     secret,
				Locked:     true,
				LockReason: LockReasonBootstrap,
			})

		case operatorSources[key] != "":
			statuses = append(statuses, KeyStatus{
				Key:        key,
				Value:      v.Get(key),
				Source:     operatorSources[key],
				Secret:     secret,
				Locked:     true,
				LockReason: LockReasonOperator,
			})

		default:
			raw, ok := dbValues[key]
			if !ok {
				statuses = append(statuses, KeyStatus{Key: key, Value: v.Get(key), Source: SourceDefault, Secret: secret})
				continue
			}

			var decoded any
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				return Config{}, nil, fmt.Errorf("config: decoding stored setting %q: %w", key, err)
			}
			v.Set(key, decoded)
			statuses = append(statuses, KeyStatus{Key: key, Value: v.Get(key), Source: SourceDB, Secret: secret})
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, nil, fmt.Errorf("config: unmarshal after overlay: %w", err)
	}
	deriveDataDirDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, nil, fmt.Errorf("config: invalid config after overlay: %w", err)
	}

	return cfg, statuses, nil
}

// operatorLockedSources reports, for every key in keys explicitly set via
// an env var or the YAML file, which of the two — env checked last so it
// overwrites yaml when both are set, matching env's higher precedence in
// Load. Keys present in neither are omitted entirely (not locked).
func operatorLockedSources(configPath string, keys []string) (map[string]Source, error) {
	sources := make(map[string]Source, len(keys))

	if configPath != "" {
		fileV := viper.New()
		fileV.SetConfigFile(configPath)
		if err := fileV.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("config: reading %s for lock detection: %w", configPath, err)
		}
		for _, key := range keys {
			if fileV.IsSet(key) {
				sources[key] = SourceYAML
			}
		}
	}

	for _, key := range keys {
		if _, ok := os.LookupEnv(envVarName(key)); ok {
			sources[key] = SourceEnv
		}
	}

	return sources, nil
}

func sourceOrDefault(sources map[string]Source, key string) Source {
	if s, ok := sources[key]; ok {
		return s
	}
	return SourceDefault
}

// allSettingValues reads every stored Setting from repo into a
// key->JSON-encoded-value map, paging through the full result set.
func allSettingValues(ctx context.Context, repo ports.SettingsRepository) (map[string]string, error) {
	values := map[string]string{}
	pageToken := ""
	for {
		settings, next, err := repo.List(ctx, 100, pageToken)
		if err != nil {
			return nil, fmt.Errorf("config: listing settings: %w", err)
		}
		for _, s := range settings {
			values[s.Key] = s.Value
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	return values, nil
}
