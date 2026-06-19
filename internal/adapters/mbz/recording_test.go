package mbz_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"testing"
)

func TestSearchItems_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":1,"recordings":[{"id":"rec-001","title":"Smells Like Teen Spirit","length":301000,"releases":[{"artist-credit":[{"name":"Nirvana","artist":{"id":"art-001","name":"Nirvana"}}]}]}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	items, err := a.SearchItems(context.Background(), domain.ContentTypeMusic, "Smells Like Teen Spirit", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.Title != "Smells Like Teen Spirit" {
		t.Errorf("Title = %q, want Smells Like Teen Spirit", item.Title)
	}
	if item.RuntimeSecs != 301 {
		t.Errorf("RuntimeSecs = %d, want 301 (301000ms / 1000)", item.RuntimeSecs)
	}
	if item.Studio == nil {
		t.Fatal("Studio should be populated from artist-credit")
	}
	if item.Studio.Name != "Nirvana" {
		t.Errorf("Studio.Name = %q, want Nirvana", item.Studio.Name)
	}
}

func TestSearchItems_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"recordings":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	items, err := a.SearchItems(context.Background(), domain.ContentTypeMusic, "nothing", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
}
