//go:build integration

package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"purser/pkg/cache"
	"purser/pkg/cache/memory"
	"sync/atomic"
	"testing"
	"time"
)

// These tests exercise the shared HTTP client (pkg/httpclient) end to end
// over a real loopback TCP connection — no fakes — both on its own and
// decorated with the shared in-memory cache (pkg/cache), per the Makefile's
// `test-integration` target (`go test -tags integration`).

func TestIntegration_ClientRoundTripsOverRealNetwork(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello from the test server"))
	}))
	defer ts.Close()

	client, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body returned error: %v", err)
	}
	if string(body) != "hello from the test server" {
		t.Fatalf("body = %q, want %q", body, "hello from the test server")
	}
}

func TestIntegration_CachingClientServesRepeatRequestsFromCache(t *testing.T) {
	var requests atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("cached payload"))
	}))
	defer ts.Close()

	client, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	c, err := memory.New("integration-test", cache.DefaultConfig())
	if err != nil {
		t.Fatalf("memory.New returned error: %v", err)
	}
	defer c.Close()

	client.Transport, err = NewCachingTransport(client.Transport, c)
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	for i := range 3 {
		resp, err := client.Get(ts.URL)
		if err != nil {
			t.Fatalf("Get #%d returned error: %v", i+1, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("reading body #%d returned error: %v", i+1, err)
		}
		if string(body) != "cached payload" {
			t.Fatalf("body #%d = %q, want %q", i+1, body, "cached payload")
		}
	}

	if requests.Load() != 1 {
		t.Fatalf("test server saw %d requests, want 1 (later Gets should be served from cache)", requests.Load())
	}
}

func TestIntegration_CachingClientRefetchesAfterEntryExpires(t *testing.T) {
	var requests atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("payload"))
	}))
	defer ts.Close()

	client, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	cfg := cache.DefaultConfig()
	cfg.DefaultTTL = 20 * time.Millisecond
	c, err := memory.New("integration-ttl-test", cfg)
	if err != nil {
		t.Fatalf("memory.New returned error: %v", err)
	}
	defer c.Close()

	client.Transport, err = NewCachingTransport(client.Transport, c)
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	resp1, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}
	resp1.Body.Close()

	time.Sleep(100 * time.Millisecond)

	resp2, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	resp2.Body.Close()

	if requests.Load() != 2 {
		t.Fatalf("test server saw %d requests, want 2 (cache entry should have expired between requests)", requests.Load())
	}
}
