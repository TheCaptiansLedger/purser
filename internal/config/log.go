package config

// Log configures the slog handler a composition root (cmd/purser)
// constructs at startup — see docs/adr/0008-structured-logging.md. Only
// cmd/purser is expected to ever read this; no other package constructs
// its own slog.Handler or calls slog.SetDefault.
type Log struct {
	// Level is the minimum slog level emitted: "debug", "info", "warn", or
	// "error".
	Level string `mapstructure:"level"`

	// Format selects the slog handler: "json" or "text".
	Format string `mapstructure:"format"`
}

// DefaultLog returns Log's defaults: info/json, matching what
// cmd/purser/serve.go already hardcodes today (slog.NewJSONHandler at
// slog.LevelInfo) — this struct exists to expose that existing choice as
// configurable, not to change current behavior. ops/purser.yaml's
// `log: level: debug, format: text` is a non-default example tuned for
// local dev, per docs/adr/0010-configuration.md.
func DefaultLog() Log {
	return Log{Level: "info", Format: "json"}
}
