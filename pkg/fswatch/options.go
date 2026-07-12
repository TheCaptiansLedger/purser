package fswatch

import (
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/pkg/fswatch"

// Option customizes a Watcher built by New. Every option defaults to
// something that works with zero configuration (global OTel providers,
// slog.Default(), a depth-based resolver from Config.CoalesceDepth), per
// ADR 0007's "no cost to opt out" principle.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	resolver       UnitResolver
}

func defaultOptions() *options {
	return &options{
		logger:         slog.Default(),
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
	}
}

// WithLogger overrides the default (slog.Default()) logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithTracerProvider overrides the default (global) TracerProvider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *options) { o.tracerProvider = tp }
}

// WithMeterProvider overrides the default (global) MeterProvider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(o *options) { o.meterProvider = mp }
}

// WithResolver overrides the default DepthResolver{Depth: cfg.CoalesceDepth}
// built from Config. Callers with irregular layouts (mixed Artist/Album and
// flat Album directories under the same root, etc.) supply their own
// UnitResolver implementation here.
func WithResolver(r UnitResolver) Option {
	return func(o *options) { o.resolver = r }
}
