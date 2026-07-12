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
	if cfg.Server.ListenAddr != ":8080" {
		t.Fatalf("Load returned ListenAddr %q, want %q", cfg.Server.ListenAddr, ":8080")
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
