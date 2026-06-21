package httpclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/pkg/httpclient"
	"testing"
)

func TestNew_Timeout(t *testing.T) {
	c := httpclient.New()
	if c.Timeout != httpclient.DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, httpclient.DefaultTimeout)
	}
}

func TestNew_TransportSet(t *testing.T) {
	c := httpclient.New()
	if c.Transport == nil {
		t.Error("Transport should not be nil")
	}
	if c.Transport == http.DefaultTransport {
		t.Error("Transport should be the logging wrapper, not http.DefaultTransport directly")
	}
}

func TestNew_RoundTripSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := httpclient.New().Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestNew_PropagatesNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := httpclient.New().Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
