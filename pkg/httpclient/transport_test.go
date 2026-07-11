package httpclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// roundTripFunc adapts a function to http.RoundTripper, used across this
// package's tests to fake the transport instrumentedTransport wraps.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func TestInstrumentedTransport_HappyPath(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(t.Context())

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer mp.Shutdown(t.Context())

	logger, buf := newTestLogger()

	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})

	o := defaultOptions()
	o.tracerProvider = tp
	o.meterProvider = mp
	o.logger = logger

	transport, err := newInstrumentedTransport(fake, "purser-test/1", o)
	if err != nil {
		t.Fatalf("newInstrumentedTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Name != "HTTP GET" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "HTTP GET")
	}

	var data metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &data); err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	found := false
	for _, sm := range data.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "http.client.request.duration" {
				found = true
			}
		}
	}
	if !found {
		t.Error("http.client.request.duration metric not recorded")
	}

	out := buf.String()
	if !strings.Contains(out, `"status":200`) {
		t.Errorf("log output missing status field: %s", out)
	}
	if !strings.Contains(out, `"trace_id"`) {
		t.Errorf("log output missing trace_id field: %s", out)
	}
}

func TestInstrumentedTransport_ErrorPath(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(t.Context())

	logger, buf := newTestLogger()

	wantErr := errors.New("boom")
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, wantErr
	})

	o := defaultOptions()
	o.tracerProvider = tp
	o.logger = logger

	transport, err := newInstrumentedTransport(fake, "", o)
	if err != nil {
		t.Fatalf("newInstrumentedTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)
	resp, err := transport.RoundTrip(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("RoundTrip error = %v, want %v", err, wantErr)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Status.Code.String() != "Error" {
		t.Errorf("span status = %v, want Error", spans[0].Status)
	}

	if !strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Errorf("log output missing ERROR level: %s", buf.String())
	}
}

func TestInstrumentedTransport_UserAgent(t *testing.T) {
	tests := []struct {
		name         string
		configuredUA string
		requestUA    string
		wantUA       string
	}{
		{"sets configured UA when absent", "purser-test/1", "", "purser-test/1"},
		{"preserves caller UA", "purser-test/1", "custom-ua/2", "custom-ua/2"},
		{"empty configured UA leaves request untouched", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUA string
			fake := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotUA = req.Header.Get("User-Agent")
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
			})

			transport, err := newInstrumentedTransport(fake, tt.configuredUA, defaultOptions())
			if err != nil {
				t.Fatalf("newInstrumentedTransport returned error: %v", err)
			}

			req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)
			if tt.requestUA != "" {
				req.Header.Set("User-Agent", tt.requestUA)
			}

			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip returned error: %v", err)
			}
			resp.Body.Close()

			if gotUA != tt.wantUA {
				t.Errorf("User-Agent = %q, want %q", gotUA, tt.wantUA)
			}
		})
	}
}

func TestInstrumentedTransport_RealNetworkRoundTrip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	transport, err := newInstrumentedTransport(http.DefaultTransport, "", defaultOptions())
	if err != nil {
		t.Fatalf("newInstrumentedTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
