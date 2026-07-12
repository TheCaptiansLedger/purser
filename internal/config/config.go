// Package config owns the single Viper instance for Purser and assembles
// component structs (Server today; more as they're needed) into one
// top-level Config tree. See docs/adr/0010-configuration.md — no other
// package imports viper directly or reads os.Getenv for application
// configuration.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level configuration tree, assembled from component
// structs.
type Config struct {
	Server Server `mapstructure:"server"`
}

// DefaultConfig returns the defaults every component starts from.
func DefaultConfig() Config {
	return Config{Server: DefaultServer()}
}

// Validate checks Config's invariants once, after Load — fail fast at
// startup rather than partway through a request, per
// docs/adr/0010-configuration.md.
func (c Config) Validate() error {
	if c.Server.ListenAddr == "" {
		return fmt.Errorf("config: server.listen_addr must not be empty")
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
	return cfg, nil
}
