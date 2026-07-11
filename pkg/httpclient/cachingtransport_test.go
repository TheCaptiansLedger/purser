package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"purser/pkg/cache"
	"purser/pkg/cache/memory"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func newTestCache(t *testing.T) cache.Cache {
	t.Helper()
	c, err := memory.New(t.Name(), cache.DefaultConfig())
	if err != nil {
		t.Fatalf("memory.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestCachingTransport_CachesGET(t *testing.T) {
	var calls atomic.Int64
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header: make(http.Header),
			Body:   io.NopCloser(strings.NewReader("body-" + strconv.FormatInt(calls.Load(), 10))),
		}, nil
	})

	c := newTestCache(t)
	transport, err := NewCachingTransport(fake, c)
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)

	resp1, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("first RoundTrip returned error: %v", err)
	}
	defer resp1.Body.Close()
	body1, _ := io.ReadAll(resp1.Body)

	resp2, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("second RoundTrip returned error: %v", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)

	if calls.Load() != 1 {
		t.Fatalf("underlying transport called %d times, want 1 (second request should be served from cache)", calls.Load())
	}
	if string(body1) != string(body2) {
		t.Errorf("cached response body = %q, want %q", body2, body1)
	}
}

func TestCachingTransport_PassesThroughNonGET(t *testing.T) {
	var calls atomic.Int64
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	c := newTestCache(t)
	transport, err := NewCachingTransport(fake, c)
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, "http://example.invalid/path", nil)
	resp1, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("first RoundTrip returned error: %v", err)
	}
	resp1.Body.Close()
	resp2, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("second RoundTrip returned error: %v", err)
	}
	resp2.Body.Close()

	if calls.Load() != 2 {
		t.Fatalf("underlying transport called %d times, want 2 (POST must never be cached)", calls.Load())
	}
}

func TestCachingTransport_DoesNotCacheNonOKStatus(t *testing.T) {
	var calls atomic.Int64
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	c := newTestCache(t)
	transport, err := NewCachingTransport(fake, c)
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)
	resp1, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("first RoundTrip returned error: %v", err)
	}
	resp1.Body.Close()
	resp2, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("second RoundTrip returned error: %v", err)
	}
	resp2.Body.Close()

	if calls.Load() != 2 {
		t.Fatalf("underlying transport called %d times, want 2 (404 responses must never be cached)", calls.Load())
	}
}

func TestCachingTransport_CorruptCacheEntryFallsThrough(t *testing.T) {
	var calls atomic.Int64
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	c := newTestCache(t)
	transport, err := NewCachingTransport(fake, c)
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)

	if err := c.Set(context.Background(), cacheKey(req), []byte("not a valid HTTP response")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	resp.Body.Close()

	if calls.Load() != 1 {
		t.Fatalf("underlying transport called %d times, want 1 (corrupt cache entry should fall through to the network)", calls.Load())
	}
}

type erroringCache struct{}

func (erroringCache) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, errors.New("boom")
}

func (erroringCache) Set(context.Context, string, []byte) error {
	return errors.New("boom")
}

func TestCachingTransport_CacheErrorsFallThroughToNetwork(t *testing.T) {
	var calls atomic.Int64
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	transport, err := NewCachingTransport(fake, erroringCache{})
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	resp.Body.Close()

	if calls.Load() != 1 {
		t.Fatalf("underlying transport called %d times, want 1", calls.Load())
	}
}

func TestCachingTransport_NetworkErrorPropagates(t *testing.T) {
	wantErr := errors.New("network unreachable")
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, wantErr
	})

	c := newTestCache(t)
	transport, err := NewCachingTransport(fake, c)
	if err != nil {
		t.Fatalf("NewCachingTransport returned error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/path", nil)
	resp, err := transport.RoundTrip(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("RoundTrip error = %v, want %v", err, wantErr)
	}
}
