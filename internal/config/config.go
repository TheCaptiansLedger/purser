package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for Purser.
type Config struct {
	Server   ServerConfig          `mapstructure:"server"`
	Database DatabaseConfig        `mapstructure:"database"`
	Media    MediaConfig           `mapstructure:"media"`
	Modules  ModulesConfig         `mapstructure:"modules"`
	Sources  MetadataSourcesConfig `mapstructure:"sources"`
	Log      LogConfig             `mapstructure:"log"`
}

// ServerConfig controls the HTTP server.
type ServerConfig struct {
	Port    int `mapstructure:"port"`
	Workers int `mapstructure:"workers"` // job queue goroutine pool size (default 4)
}

// DatabaseConfig controls the storage backend.
type DatabaseConfig struct {
	// Driver is "sqlite" (default) or "postgres".
	Driver string `mapstructure:"driver"`
	// DSN is a file path for SQLite or a connection string for PostgreSQL.
	DSN string `mapstructure:"dsn"`
}

// MediaConfig holds filesystem paths for assets managed by Purser.
type MediaConfig struct {
	// Path is the base directory for app-managed media assets (cover art, posters, headshots).
	// Purser creates subdirectories automatically: entries/, items/, people/
	// Each subdirectory is further sharded by the first two characters of the entity ID.
	Path string `mapstructure:"path"`
}

// ModulesConfig controls which content-type modules are active.
// A disabled module is invisible to both the API and the UI.
type ModulesConfig struct {
	Movies    ModuleConfig `mapstructure:"movies"`
	TV        ModuleConfig `mapstructure:"tv"`
	Music     ModuleConfig `mapstructure:"music"`
	Books     ModuleConfig `mapstructure:"books"`
	AfterDark ModuleConfig `mapstructure:"afterdark"`
	JAV       ModuleConfig `mapstructure:"jav"`
}

// ModuleConfig holds settings for a single content-type module.
type ModuleConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Roots is the list of directories to scan for existing media files of this type.
	// Multiple roots are supported for media spread across different drives or mounts.
	// Env var: PURSER_MODULES_<TYPE>_ROOTS (comma-separated paths)
	Roots []string `mapstructure:"roots"`
}

// MetadataSourcesConfig holds connection settings for all external metadata sources.
// Each source must be explicitly enabled. Sources with no API key requirement
// (MusicBrainz, OpenLibrary) are still disabled by default for operator control.
//
// Environment variable pattern: PURSER_SOURCES_<SOURCE>_<FIELD>
//
//	PURSER_SOURCES_STASHDB_ENABLED=true
//	PURSER_SOURCES_STASHDB_API_KEY=abc123
//	PURSER_SOURCES_STASH_URL=http://stash.local:9999
//	PURSER_SOURCES_STASH_API_KEY=abc123
//	PURSER_SOURCES_TMDB_API_KEY=eyJ...
//	PURSER_SOURCES_TVDB_API_KEY=abc123
//	PURSER_SOURCES_TPDB_API_KEY=abc123
//	PURSER_SOURCES_MUSICBRAINZ_ENABLED=true
//	PURSER_SOURCES_MUSICBRAINZ_USER_AGENT=myapp/1.0 (contact@example.com)
//	PURSER_SOURCES_LASTFM_API_KEY=abc123
//	PURSER_SOURCES_THEAUDIODB_ENABLED=true
//	PURSER_SOURCES_THEAUDIODB_API_KEY=123
//	PURSER_SOURCES_OPENLIBRARY_ENABLED=true
type MetadataSourcesConfig struct {
	// StashDB — adult and JAV scenes, performers, studios (stashdb.org).
	StashDB MetadataSourceConfig `mapstructure:"stashdb"`

	// TPDB — adult and JAV sites, performers, scenes (theporndb.net).
	TPDB MetadataSourceConfig `mapstructure:"tpdb"`

	// Stash — self-hosted Stash instance. URL is required.
	Stash MetadataSourceConfig `mapstructure:"stash"`

	// TMDB — movies and TV shows, cast, production companies (themoviedb.org).
	// Use the v4 "API Read Access Token" (long bearer token) as APIKey.
	TMDB MetadataSourceConfig `mapstructure:"tmdb"`

	// TVDB — TV shows, episodes, cast (thetvdb.com).
	TVDB MetadataSourceConfig `mapstructure:"tvdb"`

	// MusicBrainz — artists, releases, recordings. No API key required.
	// Set UserAgent to a descriptive string per MusicBrainz policy.
	MusicBrainz MetadataSourceConfig `mapstructure:"musicbrainz"`

	// Fanart — artist and album images for music, TV, and movies (fanart.tv). API key required.
	//  PURSER_SOURCES_FANART_API_KEY=abc123
	Fanart MetadataSourceConfig `mapstructure:"fanart"`

	// LastFM — artist and track metadata enrichment (last.fm).
	LastFM MetadataSourceConfig `mapstructure:"lastfm"`

	// TheAudioDB — artist images and album art via MBID lookup (theaudiodb.com).
	// Free tier uses API key "123".
	//  PURSER_SOURCES_THEAUDIODB_ENABLED=true
	//  PURSER_SOURCES_THEAUDIODB_API_KEY=123
	TheAudioDB MetadataSourceConfig `mapstructure:"theaudiodb"`

	// OpenLibrary — books, authors, publishers. No API key required.
	OpenLibrary MetadataSourceConfig `mapstructure:"openlibrary"`
}

// MetadataSourceConfig holds connection settings for a single external metadata source.
type MetadataSourceConfig struct {
	// Enabled controls whether Purser queries this source.
	Enabled bool `mapstructure:"enabled"`

	// URL overrides the source's well-known public endpoint.
	// Required for self-hosted sources (Stash). Leave empty for hosted services.
	URL string `mapstructure:"url"`

	// APIKey is the authentication credential for this source.
	// Exact usage varies by source (Bearer token, ApiKey header, query param, etc.).
	APIKey string `mapstructure:"api_key"`

	// UserAgent overrides the HTTP User-Agent sent to this source.
	// MusicBrainz requires a descriptive User-Agent in lieu of an API key.
	UserAgent string `mapstructure:"user_agent"`
}

// LogConfig controls log output.
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // text | json
}

// Load reads configuration from a YAML file, then overlays environment variables.
//
// Environment variables use the PURSER_ prefix; nested keys map with underscores:
//
//	PURSER_SERVER_PORT=7474
//	PURSER_DATABASE_DRIVER=sqlite
//	PURSER_DATABASE_DSN=purser.db
//	PURSER_MEDIA_PATH=./images
//	PURSER_MODULES_AFTERDARK_ENABLED=false
//	PURSER_MODULES_MOVIES_ROOTS=/mnt/disk1/movies,/mnt/disk2/movies
//	PURSER_SOURCES_STASHDB_ENABLED=true
//	PURSER_SOURCES_STASHDB_API_KEY=abc123
//	PURSER_LOG_LEVEL=info
//
// A missing file is not an error. Pass an empty path to use defaults only.
func Load(path string) (*Config, error) {
	cfg, _, _, err := LoadFull(path)
	return cfg, err
}

// LoadFull is like Load but also returns the raw Viper instance and the set of
// operator-locked keys. The Viper instance provides default-value fallback for
// ConfigService.Get. The locked set identifies keys managed by the operator
// (set via env var or YAML file) that cannot be overwritten from the UI.
//
// Viper 1.21's IsSet returns true for SetDefault keys, so locking is detected
// explicitly: env vars via os.LookupEnv, file keys via a file-only Viper scan.
func LoadFull(path string) (*Config, *viper.Viper, map[string]struct{}, error) {
	v := viper.New()

	v.SetDefault("server.port", 7474)
	v.SetDefault("server.workers", 4)
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "purser.db")
	v.SetDefault("media.path", "./images")
	v.SetDefault("modules.movies.enabled", true)
	v.SetDefault("modules.tv.enabled", true)
	v.SetDefault("modules.music.enabled", true)
	v.SetDefault("modules.books.enabled", true)
	v.SetDefault("modules.afterdark.enabled", true)
	v.SetDefault("modules.jav.enabled", true)
	v.SetDefault("sources.stashdb.enabled", false)
	v.SetDefault("sources.stashdb.url", "")
	v.SetDefault("sources.stashdb.api_key", "")
	v.SetDefault("sources.stashdb.user_agent", "")
	v.SetDefault("sources.tpdb.enabled", false)
	v.SetDefault("sources.tpdb.url", "")
	v.SetDefault("sources.tpdb.api_key", "")
	v.SetDefault("sources.tpdb.user_agent", "")
	v.SetDefault("sources.stash.enabled", false)
	v.SetDefault("sources.stash.url", "")
	v.SetDefault("sources.stash.api_key", "")
	v.SetDefault("sources.stash.user_agent", "")
	v.SetDefault("sources.tmdb.enabled", false)
	v.SetDefault("sources.tmdb.url", "")
	v.SetDefault("sources.tmdb.api_key", "")
	v.SetDefault("sources.tmdb.user_agent", "")
	v.SetDefault("sources.tvdb.enabled", false)
	v.SetDefault("sources.tvdb.url", "")
	v.SetDefault("sources.tvdb.api_key", "")
	v.SetDefault("sources.tvdb.user_agent", "")
	v.SetDefault("sources.musicbrainz.enabled", false)
	v.SetDefault("sources.musicbrainz.url", "")
	v.SetDefault("sources.musicbrainz.api_key", "")
	v.SetDefault("sources.musicbrainz.user_agent", "purser/1.0 (https://github.com/purser-app/purser)")
	v.SetDefault("sources.fanart.enabled", false)
	v.SetDefault("sources.fanart.url", "")
	v.SetDefault("sources.fanart.api_key", "")
	v.SetDefault("sources.fanart.user_agent", "")
	v.SetDefault("sources.lastfm.enabled", false)
	v.SetDefault("sources.lastfm.url", "")
	v.SetDefault("sources.lastfm.api_key", "")
	v.SetDefault("sources.lastfm.user_agent", "")
	v.SetDefault("sources.theaudiodb.enabled", false)
	v.SetDefault("sources.theaudiodb.url", "")
	v.SetDefault("sources.theaudiodb.api_key", "")
	v.SetDefault("sources.theaudiodb.user_agent", "")
	v.SetDefault("sources.openlibrary.enabled", false)
	v.SetDefault("sources.openlibrary.url", "")
	v.SetDefault("sources.openlibrary.api_key", "")
	v.SetDefault("sources.openlibrary.user_agent", "")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil, fmt.Errorf("read config: %w", err)
		}
	}

	v.SetEnvPrefix("PURSER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, nil, nil, fmt.Errorf("parse config: %w", err)
	}

	locked := computeLockedKeys(path, v.AllKeys())
	return &cfg, v, locked, nil
}

// computeLockedKeys returns the set of keys that are explicitly set by the
// operator via env vars or YAML file. Keys that only have built-in defaults
// are not locked and can be overwritten via the UI.
func computeLockedKeys(path string, allKeys []string) map[string]struct{} {
	locked := make(map[string]struct{})

	// Keys in the config file — use a file-only Viper so defaults don't bleed in.
	if path != "" {
		fileV := viper.New()
		fileV.SetConfigFile(path)
		if err := fileV.ReadInConfig(); err == nil {
			for _, k := range fileV.AllKeys() {
				locked[strings.ToLower(k)] = struct{}{}
			}
		}
	}

	// Keys set via environment variables.
	dotToUnderscore := strings.NewReplacer(".", "_")
	for _, key := range allKeys {
		envKey := "PURSER_" + strings.ToUpper(dotToUnderscore.Replace(key))
		if _, exists := os.LookupEnv(envKey); exists {
			locked[strings.ToLower(key)] = struct{}{}
		}
	}

	return locked
}
