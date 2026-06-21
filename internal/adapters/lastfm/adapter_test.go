package lastfm_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/lastfm"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
)

func newTestAdapter(srv *httptest.Server) *lastfm.Adapter {
	return lastfm.New(config.MetadataSourceConfig{APIKey: "test-key", URL: srv.URL})
}

func TestAdapter_Name(t *testing.T) {
	a := lastfm.New(config.MetadataSourceConfig{})
	if got := a.Name(); got != string(domain.SourceLastFM) {
		t.Errorf("Name() = %q, want %q", got, domain.SourceLastFM)
	}
}

func TestAdapter_ContentTypes(t *testing.T) {
	a := lastfm.New(config.MetadataSourceConfig{})
	types := a.ContentTypes()
	if len(types) != 1 || types[0] != domain.ContentTypeMusic {
		t.Errorf("ContentTypes() = %v, want [music]", types)
	}
}

func TestAdapter_FindByHash_NotSupported(t *testing.T) {
	a := lastfm.New(config.MetadataSourceConfig{})
	_, err := a.FindByHash(context.Background(), "abc123")
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

func TestAdapter_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error")) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := newTestAdapter(srv).SearchStudios(context.Background(), "test", 5)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error, got: %v", err)
	}
}

func TestAdapter_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{")) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := newTestAdapter(srv).SearchStudios(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
