// Package config owns the single Viper instance for Purser and assembles
// component structs (Server today; more as they're needed) into one
// top-level Config tree. See docs/adr/0010-configuration.md — no other
// package imports viper directly or reads os.Getenv for application
// configuration.
package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level configuration tree, assembled from component
// structs.
type Config struct {
	Server      Server      `mapstructure:"server"`
	Paths       Paths       `mapstructure:"paths"`
	Database    Database    `mapstructure:"database"`
	Media       Media       `mapstructure:"media"`
	Telemetry   Telemetry   `mapstructure:"telemetry"`
	Pipeline    Pipeline    `mapstructure:"pipeline"`
	MusicBrainz MusicBrainz `mapstructure:"musicbrainz"`
	AcoustID    AcoustID    `mapstructure:"acoustid"`
	Sources     Sources     `mapstructure:"sources"`
}

// DefaultConfig returns the defaults every component starts from.
func DefaultConfig() Config {
	cfg := Config{
		Server:      DefaultServer(),
		Paths:       DefaultPaths(),
		Database:    DefaultDatabase(),
		Media:       DefaultMedia(),
		Telemetry:   DefaultTelemetry(),
		Pipeline:    DefaultPipeline(),
		MusicBrainz: DefaultMusicBrainz(),
		AcoustID:    DefaultAcoustID(),
		Sources:     DefaultSources(),
	}
	deriveDataDirDefaults(&cfg)
	return cfg
}

// Validate checks Config's invariants once, after Load — fail fast at
// startup rather than partway through a request, per
// docs/adr/0010-configuration.md.
func (c Config) Validate() error {
	if c.Server.ListenAddr == "" {
		return fmt.Errorf("config: server.listen_addr must not be empty")
	}
	if c.Paths.DataDir == "" {
		return fmt.Errorf("config: paths.data_dir must not be empty")
	}
	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if c.Media.Path == "" {
		return fmt.Errorf("config: media.path must not be empty")
	}
	if err := c.Telemetry.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := c.Pipeline.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// Load reads configuration in precedence order: any flag already bound to
// v > the PURSER_ environment > configPath (if non-empty) > DefaultConfig.
// v is passed in (rather than using viper's global instance) so callers —
// primarily cmd/purser — control flag binding before Load runs.
func Load(v *viper.Viper, configPath string) (Config, error) {
	defaults := DefaultConfig()
	v.SetDefault("server.listen_addr", defaults.Server.ListenAddr)
	v.SetDefault("paths.data_dir", defaults.Paths.DataDir)
	v.SetDefault("database.driver", defaults.Database.Driver)
	// Registered as "" (not defaults' already-derived value) so
	// AutomaticEnv/config-file overrides on these nested keys are
	// recognized at all — Viper only resolves env vars for keys it
	// already knows about via SetDefault/BindEnv. Feeding in the derived
	// default here instead would also break re-derivation: if a caller
	// only overrides paths.data_dir, an already-derived
	// database.badger.data_dir default would shadow it. The real value
	// is filled in by deriveDataDirDefaults below, after Unmarshal.
	v.SetDefault("database.badger.data_dir", "")
	v.SetDefault("database.badger.value_log_dir", "")
	v.SetDefault("database.badger.sync_writes", false)
	v.SetDefault("database.sql.dsn", "")
	v.SetDefault("media.path", "")
	v.SetDefault("telemetry.enabled", defaults.Telemetry.Enabled)
	v.SetDefault("telemetry.otlp_endpoint", defaults.Telemetry.OTLPEndpoint)
	v.SetDefault("telemetry.otlp_insecure", defaults.Telemetry.OTLPInsecure)
	v.SetDefault("telemetry.metrics_addr", defaults.Telemetry.MetricsAddr)
	v.SetDefault("pipeline.enable_md5", defaults.Pipeline.EnableMD5)
	v.SetDefault("pipeline.enable_sha512", defaults.Pipeline.EnableSHA512)
	v.SetDefault("pipeline.scan_roots", defaults.Pipeline.ScanRoots)
	v.SetDefault("pipeline.confidence_threshold", defaults.Pipeline.ConfidenceThreshold)
	v.SetDefault("pipeline.organize", defaults.Pipeline.Organize)
	v.SetDefault("pipeline.auto_organize", defaults.Pipeline.AutoOrganize)
	v.SetDefault("musicbrainz.base_url", defaults.MusicBrainz.BaseURL)
	v.SetDefault("musicbrainz.response_header_timeout", defaults.MusicBrainz.ResponseHeaderTimeout)
	v.SetDefault("acoustid.base_url", defaults.AcoustID.BaseURL)
	v.SetDefault("acoustid.api_key", defaults.AcoustID.APIKey)
	v.SetDefault("sources.stashdb.enabled", defaults.Sources.StashDB.Enabled)
	v.SetDefault("sources.stashdb.api_key", defaults.Sources.StashDB.APIKey)
	v.SetDefault("sources.tpdb.enabled", defaults.Sources.ThePornDB.Enabled)
	v.SetDefault("sources.tpdb.api_key", defaults.Sources.ThePornDB.APIKey)

	v.SetEnvPrefix("purser")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return defaults, fmt.Errorf("config: reading %s: %w", configPath, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return defaults, fmt.Errorf("config: unmarshal: %w", err)
	}
	// Database.Badger.DataDir and Database.SQL.DSN (sqlite only) have no
	// per-component default — an empty value here means "not explicitly
	// set by flag/env/file," so they're derived from Paths.DataDir now
	// that the real value (default or overridden) is known. This
	// cross-component derivation belongs at the composition point
	// (Config), not inside either component's own DefaultConfig, per
	// docs/adr/0010-configuration.md.
	deriveDataDirDefaults(&cfg)
	return cfg, nil
}

// deriveDataDirDefaults fills in Database.Badger.DataDir,
// Database.SQL.DSN (sqlite only), and Media.Path from cfg.Paths.DataDir
// when the user hasn't explicitly set them.
func deriveDataDirDefaults(cfg *Config) {
	if cfg.Database.Badger.DataDir == "" {
		cfg.Database.Badger.DataDir = filepath.Join(cfg.Paths.DataDir, "badger")
	}
	if cfg.Database.Driver == "sqlite" && cfg.Database.SQL.DSN == "" {
		cfg.Database.SQL.DSN = filepath.Join(cfg.Paths.DataDir, "purser.db")
	}
	if cfg.Media.Path == "" {
		cfg.Media.Path = filepath.Join(cfg.Paths.DataDir, "media")
	}
}
