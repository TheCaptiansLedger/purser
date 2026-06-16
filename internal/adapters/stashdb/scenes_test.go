package stashdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"purser/internal/adapters/stashdb"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
)

const fetchStudioFixture = `{
  "data": {
    "queryScenes": {
      "count": 3,
      "scenes": [
        {
          "id": "scene-001",
          "title": "Scene One",
          "description": "First test scene",
          "date": "2023-06-15",
          "duration": 3600,
          "images": [{"url": "https://example.com/img1.jpg"}],
          "tags": [{"name": "lesbian"}, {"name": "outdoor"}],
          "studio": {"id": "studio-001", "name": "Test Studio", "parent": null},
          "performers": []
        },
        {
          "id": "scene-002",
          "title": "Scene Two",
          "description": "",
          "date": "2023-05-10",
          "duration": 1800,
          "images": [],
          "tags": [],
          "studio": null,
          "performers": []
        }
      ]
    }
  }
}`

func newTestAdapter(srv *httptest.Server) *stashdb.Adapter {
	return stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "test-key"})
}

func TestFetchEntryContent_ReturnsItemsFlat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fetchStudioFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	groups, items, total, err := a.FetchEntryContent(context.Background(), "studio-001", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if groups != nil {
		t.Errorf("expected nil groups for StashDB flat hierarchy, got %d", len(groups))
	}
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items in page, got %d", len(items))
	}

	first := items[0]
	if first.ExternalID != "scene-001" {
		t.Errorf("ExternalID: want scene-001, got %s", first.ExternalID)
	}
	if first.Title != "Scene One" {
		t.Errorf("Title: want Scene One, got %s", first.Title)
	}
	if first.ContentType != domain.ContentTypeAdult {
		t.Errorf("ContentType: want adult, got %s", first.ContentType)
	}
	if first.RuntimeSecs != 3600 {
		t.Errorf("RuntimeSecs: want 3600, got %d", first.RuntimeSecs)
	}
	if first.ImageURL != "https://example.com/img1.jpg" {
		t.Errorf("ImageURL: want https://example.com/img1.jpg, got %s", first.ImageURL)
	}
	if len(first.Tags) != 2 {
		t.Errorf("Tags: want 2, got %d", len(first.Tags))
	}
}

func TestFetchGroupContent_NotSupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // should never be called
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	_, _, err := a.FetchGroupContent(context.Background(), "any-id", 1, 100)
	if err != ports.ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}
