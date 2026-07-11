# 0007. Telemetry: OpenTelemetry Tracing and Metrics

Status: Accepted

## Context

Purser calls out to many third-party metadata providers and will grow more
adapters over time. Debugging a slow or failing lookup without traces, and
tuning cache/provider behavior without metrics, means guessing. Instrumenting
each adapter by hand, against whichever library felt convenient at the time,
would produce inconsistent signal names and make cross-cutting concerns
(shared HTTP client, shared cache — see [pkg/httpclient](../../pkg/httpclient)
and [pkg/cache](../../pkg/cache)) instrument themselves twice, once per
caller.

## Decision

- **OpenTelemetry is the only instrumentation API used in application code.**
  Library and adapter code (`pkg/**`, `internal/adapters/**`, future
  `internal/service/**`) calls `go.opentelemetry.io/otel/trace` and
  `go.opentelemetry.io/otel/metric` directly. It never imports a
  vendor-specific client (no direct `prometheus/client_golang`, no vendor
  APM SDK) and never imports the OTel *SDK* or an exporter package.
- **SDK and exporter wiring happens once, at the composition root**
  (`cmd/purser`, not yet built as of this ADR). Only `cmd/` may import
  `go.opentelemetry.io/otel/sdk/*` and exporter packages, construct a
  `TracerProvider`/`MeterProvider`, and call `otel.SetTracerProvider` /
  `otel.SetMeterProvider`. Every other package calls `otel.Tracer(name)` /
  `otel.Meter(name)` against whatever global provider the composition root
  installed — including a no-op provider if telemetry is disabled, which
  costs those callers nothing to support.
- **Traces** are exported via OTLP. **Metrics** are exposed for Prometheus
  scraping via OpenTelemetry's own Prometheus exporter
  (`go.opentelemetry.io/otel/exporters/prometheus`), which registers as a
  standard Prometheus `Collector` sourced from OTel metrics — this keeps a
  single instrumentation API instead of maintaining OTel instruments in some
  packages and raw Prometheus instruments in others.
- **Naming convention:** tracer and meter names are the importable package
  path, prefixed `purser/`, e.g. `purser/pkg/httpclient`,
  `purser/pkg/cache/memory`. Span names and metric names follow OTel semantic
  conventions where one exists for the domain (e.g. HTTP client spans/metrics
  use the `http.*` semconv attributes); where no semconv applies, names are
  `snake_case` and scoped by component (e.g. `cache.hit`, `cache.miss`).
- Every span and every recorded metric must be attributable to a specific
  component via attributes (`cache.name`, `http.host`, etc.) — a signal with
  no way to tell which instance/module produced it isn't useful in a
  multi-adapter system and isn't acceptable.

## Consequences

- Any package can be instrumented without knowing (or caring) whether traces
  end up in Jaeger, Tempo, or nowhere — it depends only on the OTel API.
- The composition root is the single place that decides sampling, batching,
  export destinations, and whether telemetry is enabled at all.
- Costs one extra hop of indirection (API vs. SDK) that must be kept straight
  in code review — an `internal/adapters` package reaching for the SDK
  directly is exactly the mistake this ADR exists to catch.

## Self-Audit Checklist

1. Does any package outside `cmd/` import `otel/sdk/*` or an exporter
   package? If yes — fix it, that wiring belongs in the composition root.
2. Does any package call a vendor-specific metrics/tracing client directly
   instead of the OTel API? If yes — fix it.
3. Does every tracer/meter obtained via `otel.Tracer`/`otel.Meter` use the
   `purser/<package path>` naming convention? If no — fix it.
4. Can every emitted span/metric be traced back to the specific
   instance/module that produced it via its attributes? If no — add the
   missing attribute.
