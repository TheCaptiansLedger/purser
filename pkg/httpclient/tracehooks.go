package httpclient

import (
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"
)

// traceHooks records per-phase timestamps via httptrace.ClientTrace's
// callbacks — the "go callback library" the shared HTTP client is
// instrumented with — so DNS lookup, connect, and TLS handshake durations
// are available for both span attributes and structured log fields on
// every request, without every caller having to wire this up itself.
type traceHooks struct {
	mu sync.Mutex

	dnsStart, dnsDone         time.Time
	connectStart, connectDone time.Time
	tlsStart, tlsDone         time.Time
}

func newTraceHooks() *traceHooks {
	return &traceHooks{}
}

func (h *traceHooks) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			h.mu.Lock()
			h.dnsStart = time.Now()
			h.mu.Unlock()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			h.mu.Lock()
			h.dnsDone = time.Now()
			h.mu.Unlock()
		},
		ConnectStart: func(string, string) {
			h.mu.Lock()
			h.connectStart = time.Now()
			h.mu.Unlock()
		},
		ConnectDone: func(string, string, error) {
			h.mu.Lock()
			h.connectDone = time.Now()
			h.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			h.mu.Lock()
			h.tlsStart = time.Now()
			h.mu.Unlock()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			h.mu.Lock()
			h.tlsDone = time.Now()
			h.mu.Unlock()
		},
	}
}

func (h *traceHooks) dnsDuration() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return phaseDuration(h.dnsStart, h.dnsDone)
}

func (h *traceHooks) connectDuration() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return phaseDuration(h.connectStart, h.connectDone)
}

func (h *traceHooks) tlsDuration() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return phaseDuration(h.tlsStart, h.tlsDone)
}

func phaseDuration(start, done time.Time) time.Duration {
	if start.IsZero() || done.IsZero() {
		return 0
	}
	return done.Sub(start)
}
