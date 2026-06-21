package theaudiodb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/domain"
	"testing"
)

const discographyFixture = `{
  "album": [
    {
      "strAlbum": "Whitesnake",
      "strMusicBrainzID": "d8cf05e8-ca97-4b13-bf05-22a5ed882bd0",
      "strAlbumThumb": "https://cdn.theaudiodb.com/images/media/album/thumb/whitesnake.jpg"
    },
    {
      "strAlbum": "Slide It In",
      "strMusicBrainzID": "e2c7bc5e-8f08-47db-aac0-f157a3d5b5b2",
      "strAlbumThumb": "https://cdn.theaudiodb.com/images/media/album/thumb/slide-it-in.jpg"
    }
  ]
}`

const discographyNullFixture = `{"album": null}`

const discographyNoThumbFixture = `{
  "album": [
    {
      "strAlbum": "No Image Album",
      "strMusicBrainzID": "some-mbid",
      "strAlbumThumb": ""
    }
  ]
}`

const discographyNoMBIDFixture = `{
  "album": [
    {
      "strAlbum": "No MBID Album",
      "strMusicBrainzID": "",
      "strAlbumThumb": "https://cdn.theaudiodb.com/images/media/album/thumb/nombid.jpg"
    }
  ]
}`

func TestFetchEntryContent_Music_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discographyFixture))
	}))
	defer srv.Close()

	_, items, total, err := newTestAdapter(srv).FetchEntryContent(context.Background(), domain.ContentTypeMusic, "artist-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
	mbid, ok := items[0].ExternalIDs["mbid"]
	if !ok || mbid != "d8cf05e8-ca97-4b13-bf05-22a5ed882bd0" {
		t.Errorf("ExternalIDs[mbid] = %q, want first album MBID", mbid)
	}
	if len(items[0].Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(items[0].Images))
	}
	if items[0].Images[0].Type != domain.ImageTypePoster {
		t.Errorf("image type = %q, want poster", items[0].Images[0].Type)
	}
	if items[0].Images[0].URL == "" {
		t.Error("image URL should be non-empty")
	}
}

func TestFetchEntryContent_Music_NullAlbums(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discographyNullFixture))
	}))
	defer srv.Close()

	_, items, total, err := newTestAdapter(srv).FetchEntryContent(context.Background(), domain.ContentTypeMusic, "unknown-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("expected 0 items, got %d (total=%d)", len(items), total)
	}
}

func TestFetchEntryContent_Music_SkipsNoThumb(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discographyNoThumbFixture))
	}))
	defer srv.Close()

	_, items, total, err := newTestAdapter(srv).FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("expected 0 items (no thumb skipped), got %d", len(items))
	}
}

func TestFetchEntryContent_Music_SkipsNoMBID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discographyNoMBIDFixture))
	}))
	defer srv.Close()

	_, items, total, err := newTestAdapter(srv).FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("expected 0 items (no MBID skipped), got %d", len(items))
	}
}

func TestFetchEntryContent_Music_Pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discographyFixture))
	}))
	defer srv.Close()

	_, items, total, err := newTestAdapter(srv).FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1 (perPage=1)", len(items))
	}
}

func TestFetchEntryContent_Music_PageBeyondEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discographyFixture))
	}))
	defer srv.Close()

	_, items, total, err := newTestAdapter(srv).FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 5, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0 (page beyond end)", len(items))
	}
}
