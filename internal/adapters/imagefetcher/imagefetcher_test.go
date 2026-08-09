package imagefetcher_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"purser/internal/adapters/imagefetcher"
	"purser/internal/ports"
	"purser/internal/ports/imagefetchertest"
	"purser/pkg/httpclient/httpmock"
	"testing"
	"time"
)

// newTestFetcher builds a Client whose HTTP calls hit real localhost
// sockets (httptest.Server, via the imagefetchertest contract suite) —
// nothing to mock, unlike newMockedTestFetcher below.
func newTestFetcher(t *testing.T) *imagefetcher.Client {
	t.Helper()
	c, err := imagefetcher.New(imagefetcher.DefaultConfig())
	if err != nil {
		t.Fatalf("imagefetcher.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newMockedTestFetcher builds a Client whose HTTP calls never leave the
// process — rt answers every request from a canned route table
// (pkg/httpclient/httpmock), plugged in via WithBaseTransport.
func newMockedTestFetcher(t *testing.T, rt http.RoundTripper, opts ...imagefetcher.Option) *imagefetcher.Client {
	t.Helper()
	allOpts := append([]imagefetcher.Option{imagefetcher.WithBaseTransport(rt)}, opts...)
	c, err := imagefetcher.New(imagefetcher.DefaultConfig(), allOpts...)
	if err != nil {
		t.Fatalf("imagefetcher.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_ImageFetcherContract(t *testing.T) {
	imagefetchertest.TestImageFetcher(t, func(t *testing.T) ports.ImageFetcher {
		return newTestFetcher(t)
	})
}

func TestFetch_CachesGETResponses(t *testing.T) {
	var hits int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/cache-test.jpg",
		Responder: func(req *http.Request) (*http.Response, error) {
			hits++
			return httpmock.Raw(http.StatusOK, []byte("bytes"))(req)
		},
	})

	c := newMockedTestFetcher(t, rt)
	ctx := context.Background()

	for range 2 {
		rc, err := c.Fetch(ctx, "http://example.invalid/cache-test.jpg")
		if err != nil {
			t.Fatalf("Fetch returned error: %v", err)
		}
		_ = rc.Close()
	}

	if hits != 1 {
		t.Errorf("server received %d requests, want 1 (second call should be served from cache)", hits)
	}
}

func TestFetch_RetriesOn503ThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/x.jpg",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return httpmock.Status(http.StatusServiceUnavailable)(req)
			}
			return httpmock.Raw(http.StatusOK, []byte("bytes"))(req)
		},
	})

	c := newMockedTestFetcher(t, rt, imagefetcher.WithRetryBaseDelay(time.Millisecond))
	rc, err := c.Fetch(context.Background(), "http://example.invalid/x.jpg")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, _ := io.ReadAll(rc)
	if string(got) != "bytes" {
		t.Errorf("body = %q, want %q", got, "bytes")
	}
	if calls != 3 {
		t.Errorf("Responder called %d times, want 3 (two 503s then a success)", calls)
	}
}

func TestFetch_GivesUpAfter503EveryAttempt(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/x.jpg",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.Status(http.StatusServiceUnavailable)(req)
		},
	})

	c := newMockedTestFetcher(t, rt, imagefetcher.WithRetryBaseDelay(time.Millisecond))
	if _, err := c.Fetch(context.Background(), "http://example.invalid/x.jpg"); err == nil {
		t.Fatal("Fetch with a persistent 503 returned nil error")
	}
	if calls != 4 {
		t.Errorf("Responder called %d times, want 4 (maxAttempts, then give up)", calls)
	}
}

func TestFetch_DoesNotRetry404(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/x.jpg",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			return httpmock.Status(http.StatusNotFound)(req)
		},
	})

	c := newMockedTestFetcher(t, rt, imagefetcher.WithRetryBaseDelay(time.Millisecond))
	if _, err := c.Fetch(context.Background(), "http://example.invalid/x.jpg"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Fetch error = %v, want ports.ErrNotFound", err)
	}
	if calls != 1 {
		t.Errorf("Responder called %d times, want 1 — a 404 is a real answer, not a transient failure", calls)
	}
}

func TestFetch_RetriesTransportTimeoutThenSucceeds(t *testing.T) {
	var calls int
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/x.jpg",
		Responder: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return httpmock.Timeout()(req)
			}
			return httpmock.Raw(http.StatusOK, []byte("bytes"))(req)
		},
	})

	c := newMockedTestFetcher(t, rt, imagefetcher.WithRetryBaseDelay(time.Millisecond))
	rc, err := c.Fetch(context.Background(), "http://example.invalid/x.jpg")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	_ = rc.Close()
	if calls != 2 {
		t.Errorf("Responder called %d times, want 2 (one simulated timeout then a success)", calls)
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	rt := httpmock.New(httpmock.Route{
		Method: http.MethodGet,
		Path:   "/x.jpg",
		Responder: func(req *http.Request) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return httpmock.Raw(http.StatusOK, []byte("bytes"))(req)
		},
	})

	cfg := imagefetcher.DefaultConfig()
	cfg.HTTPClient.UserAgent = "should-be-overwritten"
	c, err := imagefetcher.New(cfg, imagefetcher.WithBaseTransport(rt))
	if err != nil {
		t.Fatalf("imagefetcher.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	rc, err := c.Fetch(context.Background(), "http://example.invalid/x.jpg")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	_ = rc.Close()

	if gotUA == "should-be-overwritten" || gotUA == "" {
		t.Errorf("User-Agent = %q, want the fixed Purser/<version> value", gotUA)
	}
}
