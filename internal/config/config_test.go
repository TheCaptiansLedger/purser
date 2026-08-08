package config_test

import (
	"os"
	"path/filepath"
	"purser/internal/config"
	"purser/internal/domain"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestDefaultConfig_IsValid(t *testing.T) {
	if err := config.DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() returned error: %v", err)
	}
}

func TestConfig_Validate_RejectsEmptyListenAddr(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.ListenAddr = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate with empty ListenAddr did not return an error")
	}
}

func TestLoad_UsesDefaultsWithNoOverrides(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.ListenAddr != ":7474" {
		t.Fatalf("Load returned ListenAddr %q, want %q", cfg.Server.ListenAddr, ":7474")
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	t.Setenv("PURSER_SERVER_LISTEN_ADDR", ":9090")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.ListenAddr != ":9090" {
		t.Fatalf("Load returned ListenAddr %q, want %q", cfg.Server.ListenAddr, ":9090")
	}
}

func TestLoad_ConfigFileOverridesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "purser.yaml")
	if err := os.WriteFile(path, []byte("server:\n  listen_addr: \":7070\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := config.Load(viper.New(), path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.ListenAddr != ":7070" {
		t.Fatalf("Load returned ListenAddr %q, want %q", cfg.Server.ListenAddr, ":7070")
	}
}

func TestLoad_MissingConfigFileReturnsError(t *testing.T) {
	if _, err := config.Load(viper.New(), filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("Load with a missing config file did not return an error")
	}
}

func TestLoad_DerivesBadgerDataDirFromPaths(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := filepath.Join(cfg.Paths.DataDir, "badger")
	if cfg.Database.Badger.DataDir != want {
		t.Fatalf("Load derived Badger.DataDir %q, want %q", cfg.Database.Badger.DataDir, want)
	}
}

func TestLoad_BadgerDataDirFollowsOverriddenPathsDataDir(t *testing.T) {
	t.Setenv("PURSER_PATHS_DATA_DIR", "/custom/data")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := filepath.Join("/custom/data", "badger")
	if cfg.Database.Badger.DataDir != want {
		t.Fatalf("Load derived Badger.DataDir %q, want %q", cfg.Database.Badger.DataDir, want)
	}
}

func TestLoad_ExplicitBadgerDataDirOverridesDerivedDefault(t *testing.T) {
	t.Setenv("PURSER_DATABASE_BADGER_DATA_DIR", "/custom/badger")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Database.Badger.DataDir != "/custom/badger" {
		t.Fatalf("Load returned Badger.DataDir %q, want %q", cfg.Database.Badger.DataDir, "/custom/badger")
	}
}

func TestLoad_DerivesSQLiteDSNFromPathsWhenDriverIsSQLite(t *testing.T) {
	t.Setenv("PURSER_DATABASE_DRIVER", "sqlite")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := filepath.Join(cfg.Paths.DataDir, "purser.db")
	if cfg.Database.SQL.DSN != want {
		t.Fatalf("Load derived SQL.DSN %q, want %q", cfg.Database.SQL.DSN, want)
	}
}

func TestConfig_Validate_RejectsUnknownDriver(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Database.Driver = "mongodb"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate with an unknown database driver did not return an error")
	}
}

func TestConfig_Validate_RejectsPostgresWithoutDSN(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Database.Driver = "postgres"
	cfg.Database.SQL.DSN = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate with postgres driver and no DSN did not return an error")
	}
}

func TestConfig_Validate_AcceptsPostgresWithDSN(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Database.Driver = "postgres"
	cfg.Database.SQL.DSN = "postgres://user:pass@localhost/purser"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate with postgres driver and a DSN returned error: %v", err)
	}
}

func TestLoad_DerivesMediaPathFromPaths(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := filepath.Join(cfg.Paths.DataDir, "media")
	if cfg.Media.Path != want {
		t.Fatalf("Load derived Media.Path %q, want %q", cfg.Media.Path, want)
	}
}

func TestLoad_ExplicitMediaPathOverridesDerivedDefault(t *testing.T) {
	t.Setenv("PURSER_MEDIA_PATH", "/custom/media")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Media.Path != "/custom/media" {
		t.Fatalf("Load returned Media.Path %q, want %q", cfg.Media.Path, "/custom/media")
	}
}

func TestConfig_Validate_RejectsEmptyMediaPath(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Media.Path = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate with empty Media.Path did not return an error")
	}
}

func TestConfig_Validate_AcceptsDisabledTelemetryWithNoEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Telemetry.Enabled = false
	cfg.Telemetry.OTLPEndpoint = ""
	cfg.Telemetry.MetricsAddr = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate with disabled telemetry returned error: %v", err)
	}
}

func TestConfig_Validate_RejectsEnabledTelemetryWithNoOTLPEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Telemetry.Enabled = true
	cfg.Telemetry.OTLPEndpoint = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate with enabled telemetry and no OTLP endpoint did not return an error")
	}
}

func TestConfig_Validate_RejectsEnabledTelemetryWithNoMetricsAddr(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Telemetry.Enabled = true
	cfg.Telemetry.MetricsAddr = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate with enabled telemetry and no metrics addr did not return an error")
	}
}

func TestLoad_UsesDefaultTelemetrySettings(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Telemetry.Enabled {
		t.Fatal("Load returned Telemetry.Enabled=true, want false by default")
	}
	if cfg.Telemetry.OTLPEndpoint != "localhost:4317" {
		t.Fatalf("Load returned Telemetry.OTLPEndpoint %q, want %q", cfg.Telemetry.OTLPEndpoint, "localhost:4317")
	}
}

func TestLoad_EnvOverridesTelemetryEnabled(t *testing.T) {
	t.Setenv("PURSER_TELEMETRY_ENABLED", "true")
	t.Setenv("PURSER_TELEMETRY_OTLP_ENDPOINT", "tempo:4317")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Telemetry.Enabled {
		t.Fatal("Load returned Telemetry.Enabled=false, want true")
	}
	if cfg.Telemetry.OTLPEndpoint != "tempo:4317" {
		t.Fatalf("Load returned Telemetry.OTLPEndpoint %q, want %q", cfg.Telemetry.OTLPEndpoint, "tempo:4317")
	}
}

func TestLoad_UsesDefaultPipelineSettings(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Pipeline.EnableMD5 {
		t.Fatal("Load returned Pipeline.EnableMD5=true, want false by default")
	}
	if cfg.Pipeline.EnableSHA512 {
		t.Fatal("Load returned Pipeline.EnableSHA512=true, want false by default")
	}
}

func TestLoad_EnvOverridesPipelineHashToggles(t *testing.T) {
	t.Setenv("PURSER_PIPELINE_ENABLE_MD5", "true")
	t.Setenv("PURSER_PIPELINE_ENABLE_SHA512", "true")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Pipeline.EnableMD5 {
		t.Fatal("Load returned Pipeline.EnableMD5=false, want true")
	}
	if !cfg.Pipeline.EnableSHA512 {
		t.Fatal("Load returned Pipeline.EnableSHA512=false, want true")
	}
}

func TestLoad_UsesDefaultConfidenceThreshold(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Pipeline.ConfidenceThreshold != 0.75 {
		t.Fatalf("Load returned Pipeline.ConfidenceThreshold=%v, want 0.75 by default", cfg.Pipeline.ConfidenceThreshold)
	}
}

func TestLoad_EnvOverridesConfidenceThreshold(t *testing.T) {
	t.Setenv("PURSER_PIPELINE_CONFIDENCE_THRESHOLD", "0.9")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Pipeline.ConfidenceThreshold != 0.9 {
		t.Fatalf("Load returned Pipeline.ConfidenceThreshold=%v, want 0.9", cfg.Pipeline.ConfidenceThreshold)
	}
}

func TestLoad_UsesDefaultMusicBrainzResponseHeaderTimeout(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MusicBrainz.ResponseHeaderTimeout != 0 {
		t.Fatalf("Load returned MusicBrainz.ResponseHeaderTimeout=%v, want 0 (adapter default) by default", cfg.MusicBrainz.ResponseHeaderTimeout)
	}
}

func TestLoad_EnvOverridesMusicBrainzResponseHeaderTimeout(t *testing.T) {
	t.Setenv("PURSER_MUSICBRAINZ_RESPONSE_HEADER_TIMEOUT", "45s")

	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MusicBrainz.ResponseHeaderTimeout != 45*time.Second {
		t.Fatalf("Load returned MusicBrainz.ResponseHeaderTimeout=%v, want 45s", cfg.MusicBrainz.ResponseHeaderTimeout)
	}
}

func TestConfig_Validate_AcceptsAnyPipelineToggleCombination(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	cfg.Pipeline.EnableMD5 = true
	cfg.Pipeline.EnableSHA512 = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestLoad_UsesDefaultEmptyScanRoots(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Pipeline.ScanRoots) != 0 {
		t.Fatalf("Load returned %d ScanRoots, want 0 by default", len(cfg.Pipeline.ScanRoots))
	}
}

func TestLoad_ConfigFileScanRootsUnmarshalsPathContentTypePairs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "purser.yaml")
	yaml := "pipeline:\n  scan_roots:\n    - path: /media/incoming\n      content_type: music\n    - path: /media/movies\n      content_type: movie\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := config.Load(viper.New(), path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Pipeline.ScanRoots) != 2 {
		t.Fatalf("Load returned %d ScanRoots, want 2", len(cfg.Pipeline.ScanRoots))
	}
	want := []config.ScanRoot{
		{Path: "/media/incoming", ContentType: domain.ContentTypeMusic},
		{Path: "/media/movies", ContentType: domain.ContentTypeMovie},
	}
	for i, w := range want {
		if cfg.Pipeline.ScanRoots[i] != w {
			t.Errorf("ScanRoots[%d] = %+v, want %+v", i, cfg.Pipeline.ScanRoots[i], w)
		}
	}
}

func TestPipeline_Validate_RejectsScanRootMissingPath(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.ScanRoots = []config.ScanRoot{{Path: "", ContentType: domain.ContentTypeMusic}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error for scan_roots entry missing path")
	}
}

func TestPipeline_Validate_RejectsScanRootMissingContentType(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.ScanRoots = []config.ScanRoot{{Path: "/media/incoming", ContentType: ""}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error for scan_roots entry missing content_type")
	}
}

func TestPipeline_Validate_AcceptsWellFormedScanRoots(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.ScanRoots = []config.ScanRoot{{Path: "/media/incoming", ContentType: domain.ContentTypeMusic}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestLoad_UsesDefaultEmptyOrganize(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Pipeline.Organize) != 0 {
		t.Fatalf("Load returned %d Organize entries, want 0 by default", len(cfg.Pipeline.Organize))
	}
}

func TestLoad_ConfigFileOrganizeUnmarshalsRootTemplatePairs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "purser.yaml")
	yaml := "pipeline:\n  organize:\n    music:\n      root: /library/music\n      template: \"{{.ArtistName}}/{{.AlbumTitle}}/{{.TrackTitle}}{{.Ext}}\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := config.Load(viper.New(), path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := config.OrganizeConfig{Root: "/library/music", Template: "{{.ArtistName}}/{{.AlbumTitle}}/{{.TrackTitle}}{{.Ext}}"}
	got, ok := cfg.Pipeline.Organize[domain.ContentTypeMusic]
	if !ok {
		t.Fatalf("Load returned Organize %+v, missing music entry", cfg.Pipeline.Organize)
	}
	if got != want {
		t.Fatalf("Organize[music] = %+v, want %+v", got, want)
	}
}

func TestPipeline_Validate_RejectsOrganizeEntryMissingRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.Organize = map[domain.ContentType]config.OrganizeConfig{
		domain.ContentTypeMusic: {Root: "", Template: "{{.TrackTitle}}{{.Ext}}"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error for organize entry missing root")
	}
}

func TestPipeline_Validate_RejectsOrganizeEntryMissingTemplate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.Organize = map[domain.ContentType]config.OrganizeConfig{
		domain.ContentTypeMusic: {Root: "/library/music", Template: ""},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error for organize entry missing template")
	}
}

func TestPipeline_Validate_RejectsOrganizeEntryWithInvalidTemplate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.Organize = map[domain.ContentType]config.OrganizeConfig{
		domain.ContentTypeMusic: {Root: "/library/music", Template: "{{.TrackTitle"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error for organize entry with an unparseable template")
	}
}

func TestPipeline_Validate_AcceptsWellFormedOrganizeEntry(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.Organize = map[domain.ContentType]config.OrganizeConfig{
		domain.ContentTypeMusic: {Root: "/library/music", Template: "{{.ArtistName}}/{{.TrackTitle}}{{.Ext}}"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestPipeline_Validate_AcceptsOrganizeTemplateUsingDefaultFunc(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Pipeline.Organize = map[domain.ContentType]config.OrganizeConfig{
		domain.ContentTypeMusic: {Root: "/library/music", Template: `{{.Metadata.isrc | default "unknown"}}{{.Ext}}`},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for a template using the shared default() func", err)
	}
}

func TestLoad_UsesDefaultAutoOrganizeOff(t *testing.T) {
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Pipeline.AutoOrganize {
		t.Fatal("Load returned AutoOrganize=true by default, want false")
	}
}

func TestLoad_EnvOverridesAutoOrganize(t *testing.T) {
	t.Setenv("PURSER_PIPELINE_AUTO_ORGANIZE", "true")
	cfg, err := config.Load(viper.New(), "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Pipeline.AutoOrganize {
		t.Fatal("Load returned AutoOrganize=false, want true from env override")
	}
}
