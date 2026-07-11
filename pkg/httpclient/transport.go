package httpclient

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// instrumentedTransport wraps a base http.RoundTripper with a trace span
// (annotated with DNS/connect/TLS phase durations from httptrace), a
// request duration metric, and one structured log record per request — for
// free, to every caller that builds a client via New.
type instrumentedTransport struct {
	next      http.RoundTripper
	userAgent string

	logger *slog.Logger
	tracer trace.Tracer

	duration metric.Float64Histogram
}

func newInstrumentedTransport(next http.RoundTripper, userAgent string, o *options) (*instrumentedTransport, error) {
	meter := o.meterProvider.Meter(instrumentationName)
	duration, err := meter.Float64Histogram(
		"http.client.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("HTTP client request duration, by method/host/status"),
	)
	if err != nil {
		return nil, fmt.Errorf("httpclient: creating request duration histogram: %w", err)
	}

	return &instrumentedTransport{
		next:      next,
		userAgent: userAgent,
		logger:    o.logger.With("component", "httpclient"),
		tracer:    o.tracerProvider.Tracer(instrumentationName),
		duration:  duration,
	}, nil
}

// RoundTrip implements http.RoundTripper.
func (t *instrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.userAgent != "" && req.Header.Get("User-Agent") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", t.userAgent)
	}

	ctx, span := t.tracer.Start(req.Context(), "HTTP "+req.Method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("http.request.method", req.Method),
			attribute.String("server.address", req.URL.Hostname()),
			attribute.String("url.scheme", req.URL.Scheme),
		),
	)
	defer span.End()

	hooks := newTraceHooks()
	ctx = httptrace.WithClientTrace(ctx, hooks.clientTrace())
	req = req.WithContext(ctx)

	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	elapsed := time.Since(start)

	span.SetAttributes(
		attribute.Int64("http.client.dns_lookup.duration_ms", hooks.dnsDuration().Milliseconds()),
		attribute.Int64("http.client.connect.duration_ms", hooks.connectDuration().Milliseconds()),
		attribute.Int64("http.client.tls_handshake.duration_ms", hooks.tlsDuration().Milliseconds()),
	)

	logArgs := []any{
		"method", req.Method,
		"host", req.URL.Host,
		"duration_ms", elapsed.Milliseconds(),
		"dns_ms", hooks.dnsDuration().Milliseconds(),
		"connect_ms", hooks.connectDuration().Milliseconds(),
		"tls_ms", hooks.tlsDuration().Milliseconds(),
		"trace_id", span.SpanContext().TraceID().String(),
	}

	metricAttrs := []attribute.KeyValue{
		attribute.String("http.request.method", req.Method),
		attribute.String("server.address", req.URL.Hostname()),
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		metricAttrs = append(metricAttrs, attribute.Bool("error", true))
		t.duration.Record(ctx, elapsed.Seconds(), metric.WithAttributes(metricAttrs...))
		t.logger.ErrorContext(ctx, "http request failed", append(logArgs, "error", err)...)
		return resp, err
	}

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	metricAttrs = append(metricAttrs, attribute.Int("http.response.status_code", resp.StatusCode))
	t.duration.Record(ctx, elapsed.Seconds(), metric.WithAttributes(metricAttrs...))
	t.logger.InfoContext(ctx, "http request", append(logArgs, "status", resp.StatusCode)...)

	return resp, nil
}
