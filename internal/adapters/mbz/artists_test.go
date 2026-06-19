package mbz_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
)

const artistsFixture = `{
	"artists": [
		{"id": "abc-123", "name": "Fleetwood Mac", "disambiguation": "British-American rock band"},
		{"id": "def-456", "name": "Fleetwood", "disambiguation": ""}
	]
}`

const personFixture = `{
	"artists": [
		{"id": "ghi-789", "name": "Stevie Nicks", "disambiguation": "US singer-songwriter", "country": "US"}
	]
}`

func TestSearchStudios_HappyPath(t *testing.T) {
	var receivedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(artistsFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	results, err := a.SearchStudios(context.Background(), "Fleetwood Mac", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	got := results[0]
	if got.Source != domain.SourceMusicBrainz {
		t.Errorf("Source = %q, want %q", got.Source, domain.SourceMusicBrainz)
	}
	if got.ExternalID != "abc-123" {
		t.Errorf("ExternalID = %q, want abc-123", got.ExternalID)
	}
	if got.Name != "Fleetwood Mac" {
		t.Errorf("Name = %q, want Fleetwood Mac", got.Name)
	}
	if got.Overview != "British-American rock band" {
		t.Errorf("Overview = %q, want British-American rock band", got.Overview)
	}

	// type:Group must be part of the Lucene query, not a separate URL param
	if receivedQuery != "Fleetwood Mac AND type:Group" {
		t.Errorf("Lucene query = %q, want %q", receivedQuery, "Fleetwood Mac AND type:Group")
	}
}

func TestSearchStudios_TypeNotSeparateParam(t *testing.T) {
	var capturedURL *url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"artists":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, _ = a.SearchStudios(context.Background(), "test", 5)

	if capturedURL == nil {
		t.Fatal("no request received")
	}
	if capturedURL.Query().Get("type") != "" {
		t.Error("'type' must not appear as a standalone URL parameter — it belongs in the Lucene query")
	}
}

func TestSearchPeople_HappyPath(t *testing.T) {
	var receivedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(personFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	results, err := a.SearchPeople(context.Background(), "Stevie Nicks", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0]
	if got.Source != domain.SourceMusicBrainz {
		t.Errorf("Source = %q, want %q", got.Source, domain.SourceMusicBrainz)
	}
	if got.ExternalID != "ghi-789" {
		t.Errorf("ExternalID = %q, want ghi-789", got.ExternalID)
	}
	if got.Name != "Stevie Nicks" {
		t.Errorf("Name = %q, want Stevie Nicks", got.Name)
	}
	if got.Role != domain.RoleArtist {
		t.Errorf("Role = %q, want %q", got.Role, domain.RoleArtist)
	}
	if got.Metadata["nationality"] != "US" {
		t.Errorf("nationality metadata = %v, want US", got.Metadata["nationality"])
	}

	if receivedQuery != "Stevie Nicks AND type:Person" {
		t.Errorf("Lucene query = %q, want %q", receivedQuery, "Stevie Nicks AND type:Person")
	}
}
