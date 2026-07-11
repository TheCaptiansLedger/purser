// Package httpclient is the shared, instrumented HTTP client every part of
// Purser that calls a third-party provider is required to use (see
// AGENTS.md). It builds an *http.Client with secure-by-default transport
// settings and wraps it so every request gets a trace span (with DNS/
// connect/TLS phase timings), a duration metric, and a structured log
// record for free — see ADR 0007 and ADR 0008.
package httpclient

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
)

// New builds an *http.Client from cfg. The returned client's Transport is
// always the secure base transport (TLS 1.2 minimum, no certificate
// verification bypass) wrapped in the shared instrumentation, unless
// WithBaseTransport is used to substitute a different base — instrumentation
// still wraps whatever base is supplied.
func New(cfg Config, opts ...Option) (*http.Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	base := o.baseTransport
	if base == nil {
		base = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   cfg.DialTimeout,
				KeepAlive: cfg.KeepAlive,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          cfg.MaxIdleConns,
			MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
			MaxConnsPerHost:       cfg.MaxConnsPerHost,
			IdleConnTimeout:       cfg.IdleConnTimeout,
			TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
			ExpectContinueTimeout: cfg.ExpectContinueTimeout,
			ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}
	}

	transport, err := newInstrumentedTransport(base, cfg.UserAgent, o)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport:     transport,
		Timeout:       cfg.Timeout,
		CheckRedirect: maxRedirectsPolicy(cfg.MaxRedirects),
	}, nil
}

func maxRedirectsPolicy(max int) func(*http.Request, []*http.Request) error {
	return func(_ *http.Request, via []*http.Request) error {
		if len(via) >= max {
			return fmt.Errorf("httpclient: stopped after %d redirects", max)
		}
		return nil
	}
}
