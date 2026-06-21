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

const albumFixture = `{
  "album": [{
    "strAlbum": "Come an' Get It",
    "strMusicBrainzID": "2d4dfb06-6a4f-311c-88e7-4a7eae7e08f6",
    "strAlbumThumb": "https://r2.theaudiodb.com/images/media/album/thumb/sqtt0w1643729042.jpg"
  }]
}`

const albumNullFixture = `{"album": null}`

const albumNoThumbFixture = `{
  "album": [{
    "strAlbum": "No Image",
    "strMusicBrainzID": "some-mbid",
    "strAlbumThumb": ""
  }]
}`

func TestFetchGroupContent_Music_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(albumFixture))
	}))
	defer srv.Close()

	items, total, err := newTestAdapter(srv).FetchGroupContent(context.Background(), domain.ContentTypeMusic, "2d4dfb06-6a4f-311c-88e7-4a7eae7e08f6", 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 item, got %d (total=%d)", len(items), total)
	}
	mbid, ok := items[0].ExternalIDs["mbid"]
	if !ok || mbid != "2d4dfb06-6a4f-311c-88e7-4a7eae7e08f6" {
		t.Errorf("ExternalIDs[mbid] = %q, want release-group MBID", mbid)
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

func TestFetchGroupContent_Music_NullAlbum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(albumNullFixture))
	}))
	defer srv.Close()

	items, total, err := newTestAdapter(srv).FetchGroupContent(context.Background(), domain.ContentTypeMusic, "unknown-mbid", 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("expected 0 items, got %d (total=%d)", len(items), total)
	}
}

func TestFetchGroupContent_Music_NoThumb(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(albumNoThumbFixture))
	}))
	defer srv.Close()

	items, total, err := newTestAdapter(srv).FetchGroupContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("expected 0 items (no thumb skipped), got %d", len(items))
	}
}

func TestFetchGroupContent_NonMusic_NotSupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	_, _, err := newTestAdapter(srv).FetchGroupContent(context.Background(), domain.ContentTypeAdult, "any-id", 1, 1)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got: %v", err)
	}
}
