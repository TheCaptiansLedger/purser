package mbz_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/ports"
	"testing"
)

func TestFindByExternalID_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"abc-123","name":"Nirvana","disambiguation":"90s US grunge band"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	item, err := a.FindByExternalID(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ExternalID != "abc-123" {
		t.Errorf("ExternalID = %q, want abc-123", item.ExternalID)
	}
	if item.Title != "Nirvana" {
		t.Errorf("Title = %q, want Nirvana", item.Title)
	}
	if item.Overview != "90s US grunge band" {
		t.Errorf("Overview = %q, want 90s US grunge band", item.Overview)
	}
}

func TestFindByExternalID_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, err := a.FindByExternalID(context.Background(), "does-not-exist")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ports.ErrNotFound, got: %v", err)
	}
}
