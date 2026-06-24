package mbz_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"strings"
	"testing"
)

func TestAdapter_Name(t *testing.T) {
	a := mbz.New(config.MetadataSourceConfig{})
	if got := a.Name(); got != string(domain.SourceMusicBrainz) {
		t.Errorf("Name() = %q, want %q", got, domain.SourceMusicBrainz)
	}
}

func TestAdapter_ContentTypes(t *testing.T) {
	a := mbz.New(config.MetadataSourceConfig{})
	types := a.ContentTypes()
	if len(types) != 1 || types[0] != domain.ContentTypeMusic {
		t.Errorf("ContentTypes() = %v, want [music]", types)
	}
}

func TestAdapter_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error")) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, err := a.SearchStudios(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention 500, got: %v", err)
	}
}

func TestAdapter_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{")) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, err := a.SearchStudios(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAdapter_RateLimit_429_Retry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"artists":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, err := a.SearchStudios(context.Background(), "test", 5)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 HTTP calls (429 + retry), got %d", calls)
	}
}
