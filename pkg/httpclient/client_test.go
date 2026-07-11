package httpclient

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew_RoundTripsAgainstRealServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
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
}

func TestNew_SendsConfiguredUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.UserAgent = "purser-client-test/1"

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer resp.Body.Close()

	if gotUA != cfg.UserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, cfg.UserAgent)
	}
}

func TestNew_EnforcesTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Timeout = 20 * time.Millisecond

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := client.Get(ts.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("Get did not time out")
	}
}

func TestNew_MaxRedirectsStopsFollowing(t *testing.T) {
	var mux http.ServeMux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	})
	ts := httptest.NewServer(&mux)
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.MaxRedirects = 2

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := client.Get(ts.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("Get did not stop following redirects")
	}
	if !strings.Contains(err.Error(), "stopped after 2 redirects") {
		t.Errorf("error = %v, want it to mention the redirect cap", err)
	}
}

func TestNew_ZeroMaxRedirectsFollowsNone(t *testing.T) {
	var mux http.ServeMux
	redirects := 0
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		redirects++
		http.Redirect(w, r, "/", http.StatusFound)
	})
	ts := httptest.NewServer(&mux)
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.MaxRedirects = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := client.Get(ts.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("Get did not stop before following any redirect")
	}
	if redirects != 1 {
		t.Errorf("server saw %d requests, want exactly 1 (no redirect followed)", redirects)
	}
}

func TestNew_RejectsUntrustedTLSCertificate(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := client.Get(ts.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("Get succeeded against a server with an untrusted certificate; the client must not bypass verification")
	}
}

func TestNew_WithBaseTransportOverride(t *testing.T) {
	fake := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTeapot, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	client, err := New(DefaultConfig(), WithBaseTransport(fake))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := client.Get("http://example.invalid/")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d, want %d (from the overridden base transport)", resp.StatusCode, http.StatusTeapot)
	}
}
