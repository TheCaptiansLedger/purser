package theaudiodb_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/theaudiodb"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
)

func newTestAdapter(srv *httptest.Server) *theaudiodb.Adapter {
	return theaudiodb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "123"})
}

func TestAdapter_Name(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	if got := a.Name(); got != string(domain.SourceTheAudioDB) {
		t.Errorf("Name() = %q, want %q", got, domain.SourceTheAudioDB)
	}
}

func TestAdapter_ContentTypes(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	types := a.ContentTypes()
	if len(types) != 1 || types[0] != domain.ContentTypeMusic {
		t.Errorf("ContentTypes() = %v, want [music]", types)
	}
}

func TestAdapter_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	_, err := newTestAdapter(srv).FindByExternalID(context.Background(), domain.ContentTypeMusic, "any-id")
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention 500, got: %v", err)
	}
}

func TestAdapter_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not valid json {{{"))
	}))
	defer srv.Close()

	_, err := newTestAdapter(srv).FindByExternalID(context.Background(), domain.ContentTypeMusic, "any-id")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAdapter_FindByExternalID_UnknownContentType(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	_, err := a.FindByExternalID(context.Background(), domain.ContentTypeAdult, "any-id")
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

func TestAdapter_FindByHash_NotSupported(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	_, err := a.FindByHash(context.Background(), "abc123")
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

func TestAdapter_SearchPeople_NotSupported(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	_, err := a.SearchPeople(context.Background(), "query", 10)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

func TestAdapter_SearchItems_NotSupported(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	_, err := a.SearchItems(context.Background(), domain.ContentTypeMusic, "query", 10)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

func TestAdapter_FetchEntryContent_NonMusicNotSupported(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	_, _, _, err := a.FetchEntryContent(context.Background(), domain.ContentTypeAdult, "any-id", 1, 10)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

func TestAdapter_FetchGroupContent_NotSupported(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	_, _, err := a.FetchGroupContent(context.Background(), domain.ContentTypeMusic, "any-id", 1, 10)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}

func TestAdapter_FetchEntryPeople_NotSupported(t *testing.T) {
	a := theaudiodb.New(config.MetadataSourceConfig{})
	_, err := a.FetchEntryPeople(context.Background(), "any-id")
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}
