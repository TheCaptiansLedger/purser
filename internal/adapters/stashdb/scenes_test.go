package stashdb_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/stashdb"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

const fetchStudioFixture = `{
  "data": {
    "queryScenes": {
      "count": 3,
      "scenes": [
        {
          "id": "scene-001",
          "title": "Scene One",
          "details": "First test scene",
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
          "details": "",
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
	groups, items, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeAdult, "studio-001", 1, 10)
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
	_, _, err := a.FetchGroupContent(context.Background(), domain.ContentTypeAdult, "any-id", 1, 100)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}

const searchItemsFixture = `{
  "data": {
    "queryScenes": {
      "scenes": [
        {
          "id": "search-001",
          "title": "Search Result",
          "details": "Found via title search",
          "date": "2024-03-10",
          "duration": 1800,
          "images": [{"url": "https://example.com/search.jpg"}],
          "tags": [{"name": "outdoor"}],
          "studio": {"id": "s-1", "name": "Main Studio", "parent": {"id": "n-1", "name": "Network"}},
          "performers": []
        }
      ]
    }
  }
}`

func TestSearchItems_ReturnsItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchItemsFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	items, err := a.SearchItems(context.Background(), domain.ContentTypeAdult, "Search Result", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.ExternalID != "search-001" {
		t.Errorf("ExternalID = %q, want search-001", item.ExternalID)
	}
	if item.Title != "Search Result" {
		t.Errorf("Title = %q, want Search Result", item.Title)
	}
	if item.ContentType != domain.ContentTypeAdult {
		t.Errorf("ContentType = %q, want adult", item.ContentType)
	}
	if item.Studio == nil {
		t.Fatal("Studio should not be nil")
	}
	if item.Studio.ParentID != "n-1" {
		t.Errorf("Studio.ParentID = %q, want n-1", item.Studio.ParentID)
	}
}

const findByHashFoundFixture = `{
  "data": {
    "findScenesBySceneFingerprints": [
      [
        {
          "id": "hash-scene-001",
          "title": "Hash Matched Scene",
          "details": "",
          "date": "",
          "duration": 0,
          "images": [],
          "tags": [],
          "studio": null,
          "performers": []
        }
      ]
    ]
  }
}`

const findByHashNotFoundFixture = `{
  "data": {
    "findScenesBySceneFingerprints": [[]]
  }
}`

func TestFindByHash_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(findByHashFoundFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	item, err := a.FindByHash(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ExternalID != "hash-scene-001" {
		t.Errorf("ExternalID = %q, want hash-scene-001", item.ExternalID)
	}
}

func TestFindByHash_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(findByHashNotFoundFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	_, err := a.FindByHash(context.Background(), "nohash")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

const findByIDFoundFixture = `{
  "data": {
    "findScene": {
      "id": "scene-by-id-001",
      "title": "Found By ID",
      "details": "detail text",
      "date": "2023-11-20",
      "duration": 3000,
      "images": [],
      "tags": [],
      "studio": null,
      "performers": []
    }
  }
}`

const findByIDNotFoundFixture = `{
  "data": {
    "findScene": null
  }
}`

func TestFindByExternalID_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(findByIDFoundFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	item, err := a.FindByExternalID(context.Background(), domain.ContentTypeAdult, "scene-by-id-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ExternalID != "scene-by-id-001" {
		t.Errorf("ExternalID = %q, want scene-by-id-001", item.ExternalID)
	}
	if item.Title != "Found By ID" {
		t.Errorf("Title = %q, want Found By ID", item.Title)
	}
}

func TestFindByExternalID_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(findByIDNotFoundFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	_, err := a.FindByExternalID(context.Background(), domain.ContentTypeAdult, "no-such-id")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
