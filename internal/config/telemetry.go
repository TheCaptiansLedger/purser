package config

import "fmt"

// Telemetry configures the OTel SDK/exporter wiring built in cmd/purser
// per docs/adr/0007-telemetry.md — the composition root is the only
// place that reads these. Every instrumented package upstream of this
// config keeps working against the OTel API's default no-op provider
// when Enabled is false, at zero cost, per that ADR.
type Telemetry struct {
	// Enabled turns on the OTLP trace exporter and the Prometheus metrics
	// endpoint. Off by default so `go run`/`go test` outside the compose
	// stack never blocks on or logs errors about a missing collector.
	Enabled bool `mapstructure:"enabled"`

	// OTLPEndpoint is the OTLP/gRPC collector address (e.g. Tempo) traces
	// are exported to. Only used when Enabled.
	OTLPEndpoint string `mapstructure:"otlp_endpoint"`

	// OTLPInsecure disables TLS on the OTLP connection — true by default
	// since the local dev collector has no TLS in front of it.
	OTLPInsecure bool `mapstructure:"otlp_insecure"`

	// MetricsAddr is the address the Prometheus scrape endpoint
	// (/metrics) binds, separate from Server.ListenAddr so metrics scraping
	// is never mixed with the public Connect/gRPC surface.
	MetricsAddr string `mapstructure:"metrics_addr"`
}

// DefaultTelemetry returns Telemetry's defaults: disabled, pointed at the
// conventional local OTLP/gRPC and OTel-Prometheus-exporter ports so
// enabling it against the docs/adr/0018-local-development-environment.md
// compose stack needs only PURSER_TELEMETRY_ENABLED=true.
func DefaultTelemetry() Telemetry {
	return Telemetry{
		Enabled:      false,
		OTLPEndpoint: "localhost:4317",
		OTLPInsecure: true,
		MetricsAddr:  ":9464",
	}
}

// Validate checks Telemetry's invariants. Only enforced when Enabled —
// the zero-value/disabled case is always valid.
func (t Telemetry) Validate() error {
	if !t.Enabled {
		return nil
	}
	if t.OTLPEndpoint == "" {
		return fmt.Errorf("telemetry.otlp_endpoint must not be empty when telemetry.enabled is true")
	}
	if t.MetricsAddr == "" {
		return fmt.Errorf("telemetry.metrics_addr must not be empty when telemetry.enabled is true")
	}
	return nil
}
