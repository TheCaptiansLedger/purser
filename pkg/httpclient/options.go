package httpclient

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "purser/pkg/httpclient"

// Option customizes a Client built by New or a caching transport built by
// NewCachingTransport. Every option defaults to something that works with
// zero configuration (global OTel providers, slog.Default()), per ADR 0007's
// "no cost to opt out" principle.
type Option func(*options)

type options struct {
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	baseTransport  http.RoundTripper
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

// WithBaseTransport overrides the transport New builds from Config with rt.
// Used to inject a fake transport in tests, or a custom proxy/dialer setup
// New's Config doesn't cover.
func WithBaseTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.baseTransport = rt }
}
