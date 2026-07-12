package fswatch

import (
	"fmt"
	"time"
)

// Config configures a Watcher built by New. Every field has a sane default
// via DefaultConfig — callers only need to override what differs for their
// use case. Field names/tags follow the PURSER_<NESTED>_<KEY> convention
// documented in ADR 0010 for future Viper embedding.
type Config struct {
	// CoalesceDepth is how many path segments below a watched root are
	// treated as the reportable unit — see DepthResolver. Ignored if a
	// custom UnitResolver is supplied via WithResolver.
	CoalesceDepth int `mapstructure:"coalesce_depth"`

	// SettleWindow is how long a unit's subtree must be quiet before its
	// Event fires.
	SettleWindow time.Duration `mapstructure:"settle_window"`

	// MaxWait is a hard ceiling on how long a unit can be held pending
	// regardless of continued activity, so a unit that never goes quiet
	// (a slow, trickling download) still eventually gets reported.
	// Must be >= SettleWindow.
	MaxWait time.Duration `mapstructure:"max_wait"`

	// IgnoreGlobs excludes matching file names (filepath.Match against
	// the base name) from a unit's reported Files list. Matches still
	// count as activity and reset the unit's settle timer — a growing
	// *.part file must not let its unit settle prematurely just because
	// the final filename isn't reportable yet.
	IgnoreGlobs []string `mapstructure:"ignore_globs"`

	// EventBuffer is the buffer size of the channel returned by
	// Watcher.Events.
	EventBuffer int `mapstructure:"event_buffer"`
}

// DefaultConfig returns the sane defaults every Watcher starts from.
func DefaultConfig() Config {
	return Config{
		CoalesceDepth: 1,
		SettleWindow:  10 * time.Second,
		MaxWait:       5 * time.Minute,
		IgnoreGlobs:   []string{".DS_Store", "*.tmp", "*.part", "Thumbs.db"},
		EventBuffer:   256,
	}
}

// Validate checks Config's invariants.
func (c Config) Validate() error {
	if c.CoalesceDepth < 0 {
		return fmt.Errorf("fswatch: CoalesceDepth must be >= 0, got %d", c.CoalesceDepth)
	}
	if c.SettleWindow <= 0 {
		return fmt.Errorf("fswatch: SettleWindow must be > 0, got %s", c.SettleWindow)
	}
	if c.MaxWait < c.SettleWindow {
		return fmt.Errorf("fswatch: MaxWait (%s) must be >= SettleWindow (%s)", c.MaxWait, c.SettleWindow)
	}
	if c.EventBuffer < 0 {
		return fmt.Errorf("fswatch: EventBuffer must be >= 0, got %d", c.EventBuffer)
	}
	return nil
}
