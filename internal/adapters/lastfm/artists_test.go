package lastfm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/domain"
	"testing"
)

const artistSearchFixture = `{
  "results": {
    "artistmatches": {
      "artist": [
        {
          "name": "Radiohead",
          "mbid": "a74b1b7f-71a5-4011-9441-d0b5e4122711",
          "image": [
            {"#text": "", "size": "small"},
            {"#text": "https://example.com/rh_large.jpg", "size": "large"},
            {"#text": "https://example.com/rh_xl.jpg", "size": "extralarge"},
            {"#text": "", "size": "mega"}
          ]
        },
        {
          "name": "Radiohead Tribute",
          "mbid": "",
          "image": []
        }
      ]
    }
  }
}`

func TestSearchStudios_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(artistSearchFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	results, err := newTestAdapter(srv).SearchStudios(context.Background(), "Radiohead", 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	got := results[0]
	if got.Name != "Radiohead" {
		t.Errorf("Name = %q, want Radiohead", got.Name)
	}
	if got.ExternalID != "a74b1b7f-71a5-4011-9441-d0b5e4122711" {
		t.Errorf("ExternalID = %q, want Radiohead MBID", got.ExternalID)
	}
	if got.Source != domain.SourceLastFM {
		t.Errorf("Source = %q, want %q", got.Source, domain.SourceLastFM)
	}
	// bestImage should pick extralarge (index 2) over large (index 1), skipping empty mega
	if got.ImageURL != "https://example.com/rh_xl.jpg" {
		t.Errorf("ImageURL = %q, want extralarge URL", got.ImageURL)
	}
}

func TestSearchStudios_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":{"artistmatches":{"artist":[]}}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	results, err := newTestAdapter(srv).SearchStudios(context.Background(), "zzznomatch", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchStudios_PlaceholderImageSkipped(t *testing.T) {
	const fixture = `{"results":{"artistmatches":{"artist":[{
      "name": "Test", "mbid": "abc-123",
      "image": [{"#text":"https://lastfm.freetls.fastly.net/i/u/300x300/2a96cbd8b46e442fc41c2b86b821562f.png","size":"extralarge"}]
    }]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixture)) //nolint:errcheck
	}))
	defer srv.Close()

	results, err := newTestAdapter(srv).SearchStudios(context.Background(), "Test", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].ImageURL != "" {
		t.Errorf("expected empty ImageURL for placeholder, got %q", results[0].ImageURL)
	}
}
