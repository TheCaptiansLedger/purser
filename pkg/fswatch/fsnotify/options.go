package fsnotify

import "log/slog"

// Option customizes a Watcher built by New.
type Option func(*options)

type options struct {
	logger *slog.Logger
}

func defaultOptions() *options {
	return &options{logger: slog.Default()}
}

// WithLogger overrides the default (slog.Default()) logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}
