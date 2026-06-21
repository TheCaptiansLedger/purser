package theaudiodb_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

const artistFixture = `{
  "artists": [{
    "strArtist": "Whitesnake",
    "strMusicBrainzID": "5be36b67-c819-4337-bec6-b17e1cc9de70",
    "strBiographyEN": "Whitesnake are a British rock band formed in 1978.",
    "strArtistThumb": "https://cdn.theaudiodb.com/images/media/artist/thumb/whitesnake.jpg",
    "strArtistFanart": "https://cdn.theaudiodb.com/images/media/artist/fanart/whitesnake-fanart1.jpg",
    "strArtistFanart2": "https://cdn.theaudiodb.com/images/media/artist/fanart/whitesnake-fanart2.jpg",
    "strArtistFanart3": "",
    "strArtistBanner": "https://cdn.theaudiodb.com/images/media/artist/banner/whitesnake-banner.jpg",
    "strWebsite": "www.whitesnake.com",
    "strGenre": "Rock"
  }]
}`

const artistNotFoundFixture = `{"artists": null}`

func TestFindByExternalID_Music_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistFixture))
	}))
	defer srv.Close()

	item, err := newTestAdapter(srv).FindByExternalID(context.Background(), domain.ContentTypeMusic, "5be36b67-c819-4337-bec6-b17e1cc9de70")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Title != "Whitesnake" {
		t.Errorf("Title = %q, want Whitesnake", item.Title)
	}
	if item.ExternalID != "5be36b67-c819-4337-bec6-b17e1cc9de70" {
		t.Errorf("ExternalID = %q, want MBID", item.ExternalID)
	}
	if item.Source != domain.SourceTheAudioDB {
		t.Errorf("Source = %q, want %q", item.Source, domain.SourceTheAudioDB)
	}
	if item.ContentType != domain.ContentTypeMusic {
		t.Errorf("ContentType = %q, want music", item.ContentType)
	}
	if item.Overview == "" {
		t.Error("Overview should be non-empty")
	}
}

func TestFindByExternalID_Music_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistNotFoundFixture))
	}))
	defer srv.Close()

	_, err := newTestAdapter(srv).FindByExternalID(context.Background(), domain.ContentTypeMusic, "unknown-mbid")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestSearchStudios_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistFixture))
	}))
	defer srv.Close()

	results, err := newTestAdapter(srv).SearchStudios(context.Background(), "Whitesnake", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	if got.Name != "Whitesnake" {
		t.Errorf("Name = %q, want Whitesnake", got.Name)
	}
	if got.ExternalID != "5be36b67-c819-4337-bec6-b17e1cc9de70" {
		t.Errorf("ExternalID = %q, want MBID", got.ExternalID)
	}
	if got.Source != domain.SourceTheAudioDB {
		t.Errorf("Source = %q, want %q", got.Source, domain.SourceTheAudioDB)
	}
	if got.ImageURL == "" {
		t.Error("ImageURL should be non-empty")
	}
}

func TestSearchStudios_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistNotFoundFixture))
	}))
	defer srv.Close()

	results, err := newTestAdapter(srv).SearchStudios(context.Background(), "Unknown Artist", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchStudios_SkipsNoMBID(t *testing.T) {
	const fixture = `{"artists": [{"strArtist": "No MBID Artist", "strMusicBrainzID": ""}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	results, err := newTestAdapter(srv).SearchStudios(context.Background(), "No MBID Artist", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (no MBID skipped), got %d", len(results))
	}
}

func TestFindByExternalID_Music_Images(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistFixture))
	}))
	defer srv.Close()

	item, err := newTestAdapter(srv).FindByExternalID(context.Background(), domain.ContentTypeMusic, "5be36b67-c819-4337-bec6-b17e1cc9de70")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ImageURL == "" {
		t.Error("ImageURL should be non-empty")
	}
	// fixture has thumb(hero) + 2 fanarts(background) + banner; fanart3 is empty and skipped
	if len(item.Images) != 4 {
		t.Errorf("expected 4 images, got %d", len(item.Images))
	}
	counts := make(map[domain.ImageType]int)
	for _, img := range item.Images {
		counts[img.Type]++
	}
	if counts[domain.ImageTypeHero] != 1 {
		t.Errorf("expected 1 hero image, got %d", counts[domain.ImageTypeHero])
	}
	if counts[domain.ImageTypeBackground] != 2 {
		t.Errorf("expected 2 background images, got %d", counts[domain.ImageTypeBackground])
	}
	if counts[domain.ImageTypeBanner] != 1 {
		t.Errorf("expected 1 banner image, got %d", counts[domain.ImageTypeBanner])
	}
}

func TestFindByExternalID_Music_ImagesSkipsEmpty(t *testing.T) {
	const fixture = `{"artists": [{
		"strArtist": "Test",
		"strMusicBrainzID": "test-mbid",
		"strArtistThumb": "https://example.com/thumb.jpg",
		"strArtistFanart": "",
		"strArtistFanart2": "",
		"strArtistFanart3": "",
		"strArtistBanner": ""
	}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	item, err := newTestAdapter(srv).FindByExternalID(context.Background(), domain.ContentTypeMusic, "test-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(item.Images) != 1 {
		t.Errorf("expected 1 image (thumb only), got %d", len(item.Images))
	}
	if item.Images[0].Type != domain.ImageTypeHero {
		t.Errorf("expected hero image type, got %s", item.Images[0].Type)
	}
}

func TestSearchStudios_RespectsLimit(t *testing.T) {
	const fixture = `{"artists": [
		{"strArtist": "One",   "strMusicBrainzID": "mbid-1"},
		{"strArtist": "Two",   "strMusicBrainzID": "mbid-2"},
		{"strArtist": "Three", "strMusicBrainzID": "mbid-3"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	results, err := newTestAdapter(srv).SearchStudios(context.Background(), "Artist", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (limit=2), got %d", len(results))
	}
}
