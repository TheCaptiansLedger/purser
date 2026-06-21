// Package httpclient provides the shared HTTP client used by all off-box
// adapters. Using a single construction point lets the whole app share one
// transport (and therefore one connection pool) and ensures consistent
// timeouts and request logging everywhere.
package httpclient

import (
	"log/slog"
	"net/http"
	"time"
)

// DefaultTimeout is the standard read/write deadline for all outbound requests.
const DefaultTimeout = 30 * time.Second

// New returns an *http.Client configured with the standard Purser defaults:
// a 30-second timeout and a logging transport that emits a structured
// slog.Debug line for every completed round trip.
func New() *http.Client {
	return &http.Client{
		Timeout:   DefaultTimeout,
		Transport: &loggingTransport{base: http.DefaultTransport},
	}
}

type loggingTransport struct {
	base http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		slog.Error("http: outbound request failed",
			"method", req.Method,
			"host", req.URL.Host,
			"path", req.URL.Path,
			"error", err,
		)
		return nil, err
	}
	slog.Debug("http: outbound",
		"method", req.Method,
		"host", req.URL.Host,
		"path", req.URL.Path,
		"status", resp.StatusCode,
	)
	return resp, nil
}
