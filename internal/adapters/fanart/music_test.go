package fanart_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
)

const artistFixture = `{
	"name": "Test Artist",
	"mbid_id": "test-mbid-123",
	"artistthumb":      [{"id":"1","url":"https://assets.fanart.tv/thumb.jpg","likes":"0"}],
	"artistbackground": [{"id":"2","url":"https://assets.fanart.tv/bg.jpg","likes":"0"}],
	"musicbanner":      [{"id":"3","url":"https://assets.fanart.tv/banner.jpg","likes":"0"}],
	"hdmusiclogo":      [{"id":"4","url":"https://assets.fanart.tv/logo.jpg","likes":"0"}],
	"cdart":            [{"id":"5","url":"https://assets.fanart.tv/cdart.jpg","likes":"0"}]
}`

const albumsFixture = `{
	"name": "Test Artist",
	"mbid_id": "test-mbid-123",
	"albums": {
		"album-rg-001": {
			"albumcover": [{"id":"10","url":"https://assets.fanart.tv/cover1.jpg","likes":"0"}],
			"cdart":      [{"id":"11","url":"https://assets.fanart.tv/cdart1.jpg","likes":"0"}]
		},
		"album-rg-002": {
			"albumcover": [{"id":"12","url":"https://assets.fanart.tv/cover2.jpg","likes":"0"}]
		},
		"album-rg-003": {
			"cdart": [{"id":"13","url":"https://assets.fanart.tv/cdart3.jpg","likes":"0"}]
		}
	}
}`

func TestFindByExternalID_Music_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(artistFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	item, err := a.FindByExternalID(context.Background(), domain.ContentTypeMusic, "test-mbid-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ExternalID != "test-mbid-123" {
		t.Errorf("ExternalID = %q, want test-mbid-123", item.ExternalID)
	}

	byType := map[domain.ImageType]int{}
	for _, img := range item.Images {
		byType[img.Type]++
		if img.URL == "" {
			t.Errorf("image of type %q has empty URL", img.Type)
		}
	}

	if byType[domain.ImageTypeHero] != 1 {
		t.Errorf("hero count = %d, want 1 (from artistthumb)", byType[domain.ImageTypeHero])
	}
	if byType[domain.ImageTypeBanner] != 1 {
		t.Errorf("banner count = %d, want 1 (from musicbanner)", byType[domain.ImageTypeBanner])
	}
	// artistbackground, hdmusiclogo, and cdart must not appear.
	total := len(item.Images)
	if total != 2 {
		t.Errorf("total images = %d, want 2 (artistbackground, hdmusiclogo, cdart skipped)", total)
	}
}

func TestFindByExternalID_Music_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	_, err := a.FindByExternalID(context.Background(), domain.ContentTypeMusic, "no-such-mbid")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFetchEntryContent_Music_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(albumsFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	groups, items, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "test-mbid-123", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if groups != nil {
		t.Errorf("groups should be nil for fanart music albums, got %d", len(groups))
	}
	// album-rg-003 has no albumcover so it is skipped; 2 albums returned.
	if total != 2 {
		t.Errorf("total = %d, want 2 (album-rg-003 has no albumcover)", total)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for i, item := range items {
		if item.ExternalIDs["mbid"] == "" {
			t.Errorf("items[%d].ExternalIDs[mbid] is empty", i)
		}
		hasPoster := false
		for _, img := range item.Images {
			if img.Type == domain.ImageTypePoster {
				hasPoster = true
			}
			if img.URL == "" {
				t.Errorf("items[%d] has image with empty URL", i)
			}
		}
		if !hasPoster {
			t.Errorf("items[%d] has no poster image", i)
		}
	}
}

func TestFetchEntryContent_Music_SkipsCdartOnly(t *testing.T) {
	const cdartOnly = `{"name":"A","mbid_id":"x","albums":{"rg-1":{"cdart":[{"id":"1","url":"https://example.com/cdart.jpg","likes":"0"}]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cdartOnly)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	_, items, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "x", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("expected 0 items when album has only cdart, got total=%d items=%d", total, len(items))
	}
}

func TestFetchEntryContent_Music_EmptyAlbums(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"A","mbid_id":"x","albums":{}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	_, items, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "x", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("expected 0 items for empty albums response, got total=%d items=%d", total, len(items))
	}
}

func TestFetchEntryContent_Music_Pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(albumsFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)

	// page 1, perPage 1 — should return the first album (sorted by MBID)
	_, items, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "test-mbid-123", 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 1 {
		t.Fatalf("page 1 len = %d, want 1", len(items))
	}
	firstMBID := items[0].ExternalIDs["mbid"]

	// page 2, perPage 1 — should return the second album
	_, items2, _, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "test-mbid-123", 2, 1)
	if err != nil {
		t.Fatalf("page 2 error: %v", err)
	}
	if len(items2) != 1 {
		t.Fatalf("page 2 len = %d, want 1", len(items2))
	}
	if items2[0].ExternalIDs["mbid"] == firstMBID {
		t.Errorf("page 2 returned same album as page 1 (%q)", firstMBID)
	}

	// page 3 — beyond total, should return empty slice
	_, items3, _, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "test-mbid-123", 3, 1)
	if err != nil {
		t.Fatalf("page 3 error: %v", err)
	}
	if len(items3) != 0 {
		t.Errorf("page 3 len = %d, want 0", len(items3))
	}
}

func TestFetchEntryContent_Music_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	_, _, _, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "no-such-mbid", 1, 10)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFindByExternalID_Music_AllURLsHTTPS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(artistFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	item, err := a.FindByExternalID(context.Background(), domain.ContentTypeMusic, "test-mbid-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, img := range item.Images {
		if !strings.HasPrefix(img.URL, "https://") {
			t.Errorf("image URL %q does not begin with https://", img.URL)
		}
	}
}
