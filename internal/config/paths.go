package config

import (
	"os"
	"path/filepath"
)

// Paths holds the base directory Purser stores its own data under. Other
// components (Database.Badger.DataDir, Database.SQL.DSN for sqlite)
// derive their own defaults from it in Config.DefaultConfig — see that
// function's comment for why the derivation lives there and not here.
// See docs/adr/0010-configuration.md.
type Paths struct {
	// DataDir is the root directory Purser's own data lives under.
	DataDir string `mapstructure:"data_dir"`
}

// DefaultPaths returns Paths' sane default: the OS-appropriate per-user
// application data directory (os.UserConfigDir() + "/purser" — e.g.
// ~/.config/purser on Linux, ~/Library/Application Support/purser on
// macOS, %AppData%/purser on Windows).
func DefaultPaths() Paths {
	dir, err := os.UserConfigDir()
	if err != nil {
		// A locked-down environment (e.g. some containers) with no
		// resolvable user config directory — fall back to the working
		// directory rather than fail DefaultConfig itself. cmd/purser
		// logs this via a config.Validate error surface if the resulting
		// path turns out to be unusable at startup.
		dir = "."
	}
	return Paths{DataDir: filepath.Join(dir, "purser")}
}
