package fanart_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/fanart"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
)

func newTestAdapter(srv *httptest.Server) *fanart.Adapter {
	return fanart.New(config.MetadataSourceConfig{URL: srv.URL})
}

func TestAdapter_Name(t *testing.T) {
	a := fanart.New(config.MetadataSourceConfig{})
	if got := a.Name(); got != string(domain.SourceFanart) {
		t.Errorf("Name() = %q, want %q", got, domain.SourceFanart)
	}
}

func TestAdapter_ContentTypes(t *testing.T) {
	a := fanart.New(config.MetadataSourceConfig{})
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

	a := newTestAdapter(srv)
	_, err := a.FindByExternalID(context.Background(), domain.ContentTypeMusic, "any-id")
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

	a := newTestAdapter(srv)
	_, err := a.FindByExternalID(context.Background(), domain.ContentTypeMusic, "any-id")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAdapter_FindByExternalID_UnknownContentType(t *testing.T) {
	a := fanart.New(config.MetadataSourceConfig{})
	_, err := a.FindByExternalID(context.Background(), domain.ContentTypeAdult, "any-id")
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported for unsupported content type, got: %v", err)
	}
}

func TestAdapter_FetchEntryContent_UnknownContentType(t *testing.T) {
	a := fanart.New(config.MetadataSourceConfig{})
	_, _, _, err := a.FetchEntryContent(context.Background(), domain.ContentTypeAdult, "any-id", 1, 10)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported for unsupported content type, got: %v", err)
	}
}
