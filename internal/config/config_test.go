package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load empty path: %v", err)
	}
	if cfg.Server.Port != 7474 {
		t.Errorf("default port = %d, want 7474", cfg.Server.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("default driver = %q, want sqlite", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "purser.db" {
		t.Errorf("default dsn = %q, want purser.db", cfg.Database.DSN)
	}
	if cfg.Media.Path != "./images" {
		t.Errorf("default media path = %q, want ./images", cfg.Media.Path)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("default log level = %q, want info", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("default log format = %q, want text", cfg.Log.Format)
	}
}

func TestLoad_Defaults_AllModulesEnabled(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Modules.Movies.Enabled {
		t.Error("movies should be enabled by default")
	}
	if !cfg.Modules.TV.Enabled {
		t.Error("tv should be enabled by default")
	}
	if !cfg.Modules.Music.Enabled {
		t.Error("music should be enabled by default")
	}
	if !cfg.Modules.Books.Enabled {
		t.Error("books should be enabled by default")
	}
	if !cfg.Modules.AfterDark.Enabled {
		t.Error("afterdark should be enabled by default")
	}
	if !cfg.Modules.JAV.Enabled {
		t.Error("jav should be enabled by default")
	}
}

func TestLoad_Defaults_RootsEmpty(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Modules.Movies.Roots) != 0 {
		t.Errorf("movies roots should be empty by default, got %v", cfg.Modules.Movies.Roots)
	}
	if len(cfg.Modules.TV.Roots) != 0 {
		t.Errorf("tv roots should be empty by default, got %v", cfg.Modules.TV.Roots)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/no/such/file.yaml")
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if cfg.Server.Port != 7474 {
		t.Errorf("port = %d, want default 7474", cfg.Server.Port)
	}
}

func TestLoad_FromYAML(t *testing.T) {
	f := filepath.Join(t.TempDir(), "purser.yaml")
	if err := os.WriteFile(f, []byte(`
server:
  port: 9090
database:
  driver: postgres
  dsn: postgres://localhost/test
media:
  path: /var/lib/purser/images
modules:
  movies:
    enabled: true
    roots:
      - /mnt/disk1/movies
      - /mnt/disk2/movies
  afterdark:
    enabled: false
  jav:
    enabled: false
log:
  level: debug
  format: json
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load from file: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Media.Path != "/var/lib/purser/images" {
		t.Errorf("media path = %q, want /var/lib/purser/images", cfg.Media.Path)
	}
	if len(cfg.Modules.Movies.Roots) != 2 {
		t.Errorf("movies roots count = %d, want 2", len(cfg.Modules.Movies.Roots))
	}
	if cfg.Modules.Movies.Roots[0] != "/mnt/disk1/movies" {
		t.Errorf("movies roots[0] = %q, want /mnt/disk1/movies", cfg.Modules.Movies.Roots[0])
	}
	if cfg.Modules.AfterDark.Enabled {
		t.Error("afterdark should be disabled")
	}
	if cfg.Modules.JAV.Enabled {
		t.Error("jav should be disabled")
	}
	if cfg.Modules.Movies.Enabled != true {
		t.Error("movies should remain enabled (default)")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("log level = %q, want debug", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("log format = %q, want json", cfg.Log.Format)
	}
}

func TestLoad_Defaults_SourcesDisabled(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sources.StashDB.Enabled {
		t.Error("stashdb should be disabled by default")
	}
	if cfg.Sources.TPDB.Enabled {
		t.Error("tpdb should be disabled by default")
	}
	if cfg.Sources.Stash.Enabled {
		t.Error("stash should be disabled by default")
	}
	if cfg.Sources.TMDB.Enabled {
		t.Error("tmdb should be disabled by default")
	}
	if cfg.Sources.TVDB.Enabled {
		t.Error("tvdb should be disabled by default")
	}
	if cfg.Sources.MusicBrainz.Enabled {
		t.Error("musicbrainz should be disabled by default")
	}
	if cfg.Sources.LastFM.Enabled {
		t.Error("lastfm should be disabled by default")
	}
	if cfg.Sources.OpenLibrary.Enabled {
		t.Error("openlibrary should be disabled by default")
	}
	if cfg.Sources.MusicBrainz.UserAgent == "" {
		t.Error("musicbrainz user_agent should have a default value")
	}
}

func TestLoad_SourcesFromYAML(t *testing.T) {
	f := filepath.Join(t.TempDir(), "purser.yaml")
	if err := os.WriteFile(f, []byte(`
sources:
  stashdb:
    enabled: true
    api_key: stash-key-123
  tmdb:
    enabled: true
    api_key: tmdb-bearer-token
  musicbrainz:
    enabled: true
    user_agent: myapp/2.0 (me@example.com)
  stash:
    enabled: true
    url: http://stash.local:9999
    api_key: stash-local-key
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Sources.StashDB.Enabled {
		t.Error("stashdb should be enabled")
	}
	if cfg.Sources.StashDB.APIKey != "stash-key-123" {
		t.Errorf("stashdb api_key = %q, want stash-key-123", cfg.Sources.StashDB.APIKey)
	}
	if !cfg.Sources.TMDB.Enabled {
		t.Error("tmdb should be enabled")
	}
	if cfg.Sources.TMDB.APIKey != "tmdb-bearer-token" {
		t.Errorf("tmdb api_key = %q, want tmdb-bearer-token", cfg.Sources.TMDB.APIKey)
	}
	if cfg.Sources.MusicBrainz.UserAgent != "myapp/2.0 (me@example.com)" {
		t.Errorf("musicbrainz user_agent = %q, want myapp/2.0", cfg.Sources.MusicBrainz.UserAgent)
	}
	if cfg.Sources.Stash.URL != "http://stash.local:9999" {
		t.Errorf("stash url = %q, want http://stash.local:9999", cfg.Sources.Stash.URL)
	}
	if !cfg.Sources.TPDB.Enabled == false {
		t.Error("tpdb should remain disabled (not in yaml)")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("PURSER_SERVER_PORT", "8080")
	t.Setenv("PURSER_DATABASE_DRIVER", "postgres")
	t.Setenv("PURSER_DATABASE_DSN", "postgres://test")
	t.Setenv("PURSER_MEDIA_PATH", "/var/images")
	t.Setenv("PURSER_MODULES_AFTERDARK_ENABLED", "false")
	t.Setenv("PURSER_LOG_LEVEL", "warn")
	t.Setenv("PURSER_LOG_FORMAT", "json")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with env: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "postgres://test" {
		t.Errorf("dsn = %q, want postgres://test", cfg.Database.DSN)
	}
	if cfg.Media.Path != "/var/images" {
		t.Errorf("media path = %q, want /var/images", cfg.Media.Path)
	}
	if cfg.Modules.AfterDark.Enabled {
		t.Error("afterdark should be disabled via env")
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log level = %q, want warn", cfg.Log.Level)
	}
}

func TestLoad_SourcesEnvOverride(t *testing.T) {
	t.Setenv("PURSER_SOURCES_STASHDB_ENABLED", "true")
	t.Setenv("PURSER_SOURCES_STASHDB_API_KEY", "env-stash-key")
	t.Setenv("PURSER_SOURCES_TMDB_ENABLED", "true")
	t.Setenv("PURSER_SOURCES_TMDB_API_KEY", "env-tmdb-token")
	t.Setenv("PURSER_SOURCES_STASH_URL", "http://stash.local:9999")
	t.Setenv("PURSER_SOURCES_MUSICBRAINZ_USER_AGENT", "testapp/1.0")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with env: %v", err)
	}
	if !cfg.Sources.StashDB.Enabled {
		t.Error("stashdb should be enabled via env")
	}
	if cfg.Sources.StashDB.APIKey != "env-stash-key" {
		t.Errorf("stashdb api_key = %q, want env-stash-key", cfg.Sources.StashDB.APIKey)
	}
	if !cfg.Sources.TMDB.Enabled {
		t.Error("tmdb should be enabled via env")
	}
	if cfg.Sources.TMDB.APIKey != "env-tmdb-token" {
		t.Errorf("tmdb api_key = %q, want env-tmdb-token", cfg.Sources.TMDB.APIKey)
	}
	if cfg.Sources.Stash.URL != "http://stash.local:9999" {
		t.Errorf("stash url = %q, want http://stash.local:9999", cfg.Sources.Stash.URL)
	}
	if cfg.Sources.MusicBrainz.UserAgent != "testapp/1.0" {
		t.Errorf("musicbrainz user_agent = %q, want testapp/1.0", cfg.Sources.MusicBrainz.UserAgent)
	}
}
