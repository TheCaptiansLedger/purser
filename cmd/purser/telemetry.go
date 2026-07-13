package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"purser/internal/config"
	"purser/internal/version"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

// setupTelemetry constructs and installs the OTel SDK's TracerProvider
// (OTLP/gRPC exporter, per docs/adr/0018-local-development-environment.md's
// Tempo) and MeterProvider (Prometheus exporter, served on its own
// listener) as the process-wide globals — the only place in the codebase
// allowed to do so, per docs/adr/0007-telemetry.md. Every instrumented
// package upstream keeps calling otel.Tracer/otel.Meter against whatever
// this installs, including OTel's own no-op default when cfg.Enabled is
// false, at zero cost to those callers.
//
// The returned shutdown func flushes exporters and stops the metrics
// listener; the caller must invoke it during graceful shutdown.
func setupTelemetry(ctx context.Context, cfg config.Telemetry, logger *slog.Logger) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if !cfg.Enabled {
		return noop, nil
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", "purser"),
		attribute.String("service.version", version.Version),
	))
	if err != nil {
		return noop, fmt.Errorf("cmd/purser: building telemetry resource: %w", err)
	}

	traceOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint)}
	if cfg.OTLPInsecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}
	traceExporter, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return noop, fmt.Errorf("cmd/purser: constructing OTLP trace exporter: %w", err)
	}
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otelprom.New()
	if err != nil {
		return noop, fmt.Errorf("cmd/purser: constructing Prometheus metric exporter: %w", err)
	}
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metricExporter),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	metricsSrv := &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           promhttp.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("metrics endpoint starting", "addr", cfg.MetricsAddr)
		if serveErr := metricsSrv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("metrics endpoint failed", "error", serveErr)
		}
	}()

	return func(shutdownCtx context.Context) error {
		if shutdownErr := metricsSrv.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("cmd/purser: shutting down metrics endpoint: %w", shutdownErr)
		}
		if shutdownErr := tracerProvider.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("cmd/purser: shutting down tracer provider: %w", shutdownErr)
		}
		if shutdownErr := meterProvider.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("cmd/purser: shutting down meter provider: %w", shutdownErr)
		}
		return nil
	}, nil
}
