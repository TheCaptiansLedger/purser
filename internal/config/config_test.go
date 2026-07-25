package config_test

import (
	"os"
	"path/filepath"
	"purser/internal/config"
	"testing"

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
