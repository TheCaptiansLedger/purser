package stashdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/stashdb"
	"purser/internal/config"
	"testing"
)

const searchStudiosFixture = `{
  "data": {
    "searchStudio": [
      {
        "id": "studio-001",
        "name": "Test Studio",
        "urls": [
          {"url": "https://studio.example.com", "type": "HOME"},
          {"url": "https://other.example.com", "type": "OTHER"}
        ],
        "images": [{"url": "https://example.com/studio.jpg"}],
        "parent": null
      },
      {
        "id": "studio-002",
        "name": "Another Studio",
        "urls": [],
        "images": [],
        "parent": null
      }
    ]
  }
}`

const searchStudiosWithParentFixture = `{
  "data": {
    "searchStudio": [
      {
        "id": "studio-child-001",
        "name": "Child Studio",
        "urls": [{"url": "https://child.example.com", "type": "OTHER"}],
        "images": [],
        "parent": {
          "id": "network-001",
          "name": "Parent Network",
          "images": [{"url": "https://example.com/parent.jpg"}],
          "urls": [{"url": "https://parent.example.com", "type": "HOME"}]
        }
      }
    ]
  }
}`

func newStudioTestAdapter(srv *httptest.Server) *stashdb.Adapter {
	return stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "test-key"})
}

func TestSearchStudios_Basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchStudiosFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	studios, err := newStudioTestAdapter(srv).SearchStudios(context.Background(), "Test", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(studios) != 2 {
		t.Fatalf("expected 2 studios, got %d", len(studios))
	}

	s := studios[0]
	if s.ExternalID != "studio-001" {
		t.Errorf("ExternalID = %q, want studio-001", s.ExternalID)
	}
	if s.Name != "Test Studio" {
		t.Errorf("Name = %q, want Test Studio", s.Name)
	}
	if s.ImageURL != "https://example.com/studio.jpg" {
		t.Errorf("ImageURL = %q, want https://example.com/studio.jpg", s.ImageURL)
	}
	// HOME URL should be preferred
	if s.WebsiteURL != "https://studio.example.com" {
		t.Errorf("WebsiteURL = %q, want https://studio.example.com", s.WebsiteURL)
	}
}

func TestSearchStudios_LimitTrimsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchStudiosFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	studios, err := newStudioTestAdapter(srv).SearchStudios(context.Background(), "Test", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(studios) != 1 {
		t.Errorf("expected 1 studio after limit=1, got %d", len(studios))
	}
}

func TestSearchStudios_WithParent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchStudiosWithParentFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	studios, err := newStudioTestAdapter(srv).SearchStudios(context.Background(), "Child", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(studios) != 1 {
		t.Fatalf("expected 1 studio, got %d", len(studios))
	}

	s := studios[0]
	if s.ParentID != "network-001" {
		t.Errorf("ParentID = %q, want network-001", s.ParentID)
	}
	if s.ParentName != "Parent Network" {
		t.Errorf("ParentName = %q, want Parent Network", s.ParentName)
	}
	if s.ParentImageURL != "https://example.com/parent.jpg" {
		t.Errorf("ParentImageURL = %q, want https://example.com/parent.jpg", s.ParentImageURL)
	}
	if s.ParentWebsiteURL != "https://parent.example.com" {
		t.Errorf("ParentWebsiteURL = %q, want https://parent.example.com", s.ParentWebsiteURL)
	}
	// Non-HOME URL should still be set when it is the only one
	if s.WebsiteURL != "https://child.example.com" {
		t.Errorf("WebsiteURL = %q, want https://child.example.com", s.WebsiteURL)
	}
}

func TestSearchStudios_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newStudioTestAdapter(srv).SearchStudios(context.Background(), "any", 0)
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}
