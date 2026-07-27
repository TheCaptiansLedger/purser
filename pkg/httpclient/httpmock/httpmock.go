// Package httpmock is a canned-response http.RoundTripper: a fixed route
// table of method+path(+query) to a Responder, with no knowledge of any
// specific provider. Plug a *Transport into pkg/httpclient.WithBaseTransport
// (or an adapter's own WithBaseTransport passthrough option, e.g.
// internal/adapters/musicbrainz.WithBaseTransport) and the real adapter —
// real rate limiter, real cache, real error mapping, real JSON decoding —
// runs unmodified against fixed, known data instead of a live network call.
// Neither the adapter nor pkg/httpclient's own instrumentation knows the
// difference; RoundTrip is the only seam.
//
// This is the shared mechanism for what
// docs/adr/0003-go-testing-standards.md calls "recorded request/response
// fixtures" — one package instead of every HTTP-calling adapter growing its
// own httptest.Server-based fixture rig, and the same mechanism a
// composition root (cmd/purser) can use to run CI against fixed data with
// zero live network calls, not just unit tests.
package httpmock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// Responder produces the (*http.Response, error) RoundTrip returns for a
// matched request. It's a plain function so any outcome a real call could
// produce — a canned success, a canned error status, or a transport-level
// failure like a timeout or connection refusal — is just a value, not a
// case Transport has to special-case.
type Responder func(req *http.Request) (*http.Response, error)

// JSON replies with status and body marshaled as JSON — the common case
// for a canned provider response.
func JSON(status int, body any) Responder {
	return func(req *http.Request) (*http.Response, error) {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("httpmock: marshaling canned JSON response: %w", err)
		}
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(data)),
			Request:    req,
		}, nil
	}
}

// Raw replies with status and body verbatim, tagged Content-Type
// "application/json" — for previously recorded, already-encoded fixture
// bytes (docs/adr/0003-go-testing-standards.md's "recorded request/response
// fixtures" golden files), as opposed to JSON's marshal-a-Go-value case.
func Raw(status int, body []byte) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    req,
		}, nil
	}
}

// Status replies with status and an empty body — a bare error status (404,
// 503) that carries no payload the caller inspects.
func Status(status int) Responder {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    req,
		}, nil
	}
}

// Err fails the round trip with err itself — simulating a transport-level
// failure (connection refused, DNS failure) rather than a valid HTTP
// response ever reaching the caller.
func Err(err error) Responder {
	return func(*http.Request) (*http.Response, error) {
		return nil, err
	}
}

// timeoutError implements net.Error with Timeout() == true, the same shape
// http.Client actually returns when a request exceeds its deadline — so
// adapter code branching on a timeout sees a realistic error, without a
// real clock wait.
type timeoutError struct{ msg string }

func (e *timeoutError) Error() string   { return e.msg }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

// Timeout fails the round trip with a simulated timeout. No real wait.
func Timeout() Responder {
	return Err(&timeoutError{msg: "httpmock: simulated timeout"})
}

// Route matches one canned request to a Responder.
type Route struct {
	Method string
	Path   string

	// Query, if non-nil, must be a subset of the request's actual query
	// params: every key here must be present on the request with the same
	// value. Extra params on the request (e.g. MusicBrainz's fmt=json on
	// every request) are ignored. Nil means "don't check query params at
	// all".
	Query map[string]string

	Responder Responder
}

func (r Route) matches(req *http.Request) bool {
	if req.Method != r.Method || req.URL.Path != r.Path {
		return false
	}
	if r.Query == nil {
		return true
	}
	q := req.URL.Query()
	for k, v := range r.Query {
		if q.Get(k) != v {
			return false
		}
	}
	return true
}

// Transport is an http.RoundTripper that answers exclusively from a fixed
// Route table — no real socket, no real network, ever. Safe for concurrent
// use. An unmatched request is a hard failure (not a canned 404) so a typo
// in a fixture doesn't silently masquerade as a legitimate "not found"
// response; register an explicit Status(http.StatusNotFound) route for
// that case instead.
type Transport struct {
	routes []Route

	mu    sync.Mutex
	calls []*http.Request
}

var _ http.RoundTripper = (*Transport)(nil)

// New builds a Transport from routes, matched in order — the first
// matching Route wins.
func New(routes ...Route) *Transport {
	return &Transport{routes: routes}
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.calls = append(t.calls, req)
	t.mu.Unlock()

	for _, route := range t.routes {
		if route.matches(req) {
			return route.Responder(req)
		}
	}
	return nil, fmt.Errorf("httpmock: no route registered for %s %s", req.Method, req.URL.Path)
}

// Calls returns every request RoundTrip has received so far, in order — for
// tests asserting an adapter issued the requests it should have (e.g. rate
// limiting, caching behavior), not just that it mapped a response
// correctly.
func (t *Transport) Calls() []*http.Request {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*http.Request, len(t.calls))
	copy(out, t.calls)
	return out
}
