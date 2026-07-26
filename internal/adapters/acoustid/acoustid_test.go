package acoustid_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"purser/internal/adapters/acoustid"
	"purser/internal/ports"
	"purser/internal/ports/acoustidtest"
	"purser/internal/version"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

func newTestClient(t *testing.T, baseURL string) *acoustid.Client {
	t.Helper()
	cfg := acoustid.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = "test-key"
	c, err := acoustid.New(cfg)
	if err != nil {
		t.Fatalf("acoustid.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient_AcoustIDClientContract(t *testing.T) {
	acoustidtest.TestAcoustIDClient(t, func(t *testing.T, baseURL string) ports.AcoustIDClient {
		return newTestClient(t, baseURL)
	})
}

func TestNew_RejectsEmptyBaseURL(t *testing.T) {
	cfg := acoustid.DefaultConfig()
	cfg.BaseURL = ""
	cfg.APIKey = "test-key"
	if _, err := acoustid.New(cfg); err == nil {
		t.Fatal("acoustid.New with empty BaseURL returned nil error")
	}
}

func TestNew_RejectsEmptyAPIKey(t *testing.T) {
	cfg := acoustid.DefaultConfig()
	cfg.APIKey = ""
	if _, err := acoustid.New(cfg); err == nil {
		t.Fatal("acoustid.New with empty APIKey returned nil error")
	}
}

func TestNew_RejectsNonPositiveRequestsPerSecond(t *testing.T) {
	cfg := acoustid.DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.RequestsPerSecond = 0
	if _, err := acoustid.New(cfg); err == nil {
		t.Fatal("acoustid.New with RequestsPerSecond=0 returned nil error")
	}
}

func TestNew_SetsFixedUserAgentRegardlessOfConfig(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","results":[]}`))
	}))
	defer server.Close()

	cfg := acoustid.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.APIKey = "test-key"
	cfg.HTTPClient.UserAgent = "some-caller-supplied-value/9.9"
	c, err := acoustid.New(cfg)
	if err != nil {
		t.Fatalf("acoustid.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Lookup(context.Background(), "some-fp", 200); err == nil {
		t.Fatal("Lookup returned nil error, want ErrNotFound for empty results")
	}

	want := "Purser/" + version.Version
	if gotUA != want {
		t.Errorf("User-Agent = %q, want %q (caller-supplied value must be ignored)", gotUA, want)
	}
}

func TestLookup_SendsAPIKeyAndDuration(t *testing.T) {
	var gotClient, gotDuration, gotMeta string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClient = r.URL.Query().Get("client")
		gotDuration = r.URL.Query().Get("duration")
		gotMeta = r.URL.Query().Get("meta")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","results":[]}`))
	}))
	defer server.Close()

	cfg := acoustid.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.APIKey = "my-real-key"
	c, err := acoustid.New(cfg)
	if err != nil {
		t.Fatalf("acoustid.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Lookup(context.Background(), "some-fp", 200.6); err == nil {
		t.Fatal("Lookup returned nil error, want ErrNotFound for empty results")
	}

	if gotClient != "my-real-key" {
		t.Errorf("client param = %q, want %q", gotClient, "my-real-key")
	}
	if gotDuration != "201" {
		t.Errorf("duration param = %q, want %q (rounded)", gotDuration, "201")
	}
	if gotMeta != "recordings+releasegroups" {
		t.Errorf("meta param = %q, want %q", gotMeta, "recordings+releasegroups")
	}
}

func TestNew_WithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","results":[]}`))
	}))
	defer server.Close()

	cfg := acoustid.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.APIKey = "test-key"
	c, err := acoustid.New(cfg,
		acoustid.WithLogger(slog.Default()),
		acoustid.WithTracerProvider(otel.GetTracerProvider()),
		acoustid.WithMeterProvider(otel.GetMeterProvider()),
	)
	if err != nil {
		t.Fatalf("acoustid.New with options returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Lookup(context.Background(), "some-fp", 200); err == nil {
		t.Fatal("Lookup returned nil error, want ErrNotFound for empty results")
	}
}

func TestLookup_MapsProviderErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"error","error":{"code":4,"message":"invalid API key"}}`))
	}))
	defer server.Close()

	c := newTestClient(t, server.URL)
	_, err := c.Lookup(context.Background(), "some-fp", 200)
	if err == nil {
		t.Fatal("Lookup returned nil error, want an error for status=error")
	}
}

func TestClient_CachesGETResponses(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","results":[{"id":"x","score":0.9,"recordings":[]}]}`))
	}))
	defer server.Close()

	c := newTestClient(t, server.URL)
	ctx := context.Background()

	if _, err := c.Lookup(ctx, "cache-test-fp", 200); err != nil {
		t.Fatalf("first Lookup returned error: %v", err)
	}
	if _, err := c.Lookup(ctx, "cache-test-fp", 200); err != nil {
		t.Fatalf("second Lookup returned error: %v", err)
	}

	if hits != 1 {
		t.Errorf("server received %d requests, want 1 (second call should be served from cache)", hits)
	}
}

func TestClient_RateLimiterSerializesConcurrentRequests(t *testing.T) {
	var mu sync.Mutex
	var arrivals []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		arrivals = append(arrivals, time.Now())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","results":[{"id":"x","score":0.9,"recordings":[]}]}`))
	}))
	defer server.Close()

	cfg := acoustid.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.APIKey = "test-key"
	cfg.RequestsPerSecond = 1
	c, err := acoustid.New(cfg)
	if err != nil {
		t.Fatalf("acoustid.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	const n = 3
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Distinct fingerprints so the caching transport doesn't
			// short-circuit requests 2 and 3 against request 1's cached
			// response.
			fp := fmt.Sprintf("rate-limit-test-%d", i)
			if _, err := c.Lookup(context.Background(), fp, 200); err != nil {
				t.Errorf("Lookup returned error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	minExpected := time.Duration(n-1) * 900 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("elapsed = %v, want at least %v (rate limiter should serialize concurrent requests to 1/sec)", elapsed, minExpected)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(arrivals) != n {
		t.Fatalf("server received %d requests, want %d", len(arrivals), n)
	}
}

// requireFpcalc skips the test if the real fpcalc binary isn't on PATH —
// Fingerprint shells out to it directly (see docs/adr/0003-go-testing-
// standards.md's treatment of a local, fast, non-network binary: no
// `live` build-tag gating, but CI/dev environments without the media
// toolchain installed shouldn't fail the whole suite).
func requireFpcalc(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("fpcalc"); err != nil {
		t.Skip("fpcalc not found on PATH, skipping")
	}
}

func TestClient_Fingerprint_RealBinaryAgainstSampleAudio(t *testing.T) {
	requireFpcalc(t)
	c := newTestClient(t, "http://unused.invalid")

	fingerprint, duration, err := c.Fingerprint(context.Background(), "testdata/sample.flac")
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if fingerprint == "" {
		t.Error("fingerprint is empty, want a non-empty Chromaprint fingerprint")
	}
	if duration < 2.5 || duration > 3.5 {
		t.Errorf("duration = %v, want ~3 seconds", duration)
	}
}

func TestClient_Fingerprint_ReturnsErrorForMissingFile(t *testing.T) {
	requireFpcalc(t)
	c := newTestClient(t, "http://unused.invalid")

	if _, _, err := c.Fingerprint(context.Background(), "testdata/does-not-exist.flac"); err == nil {
		t.Fatal("Fingerprint returned nil error for a missing file")
	}
}
