package mbz_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"strings"
	"testing"
)

const twoDiscRelease = `{
	"media": [
		{
			"track-count": 2,
			"tracks": [
				{"title":"Track One","recording":{"id":"r-001","title":"Track One","length":180000}},
				{"title":"Track Two","recording":{"id":"r-002","title":"Track Two","length":240000}}
			]
		},
		{
			"track-count": 1,
			"tracks": [
				{"title":"","recording":{"id":"r-003","title":"Bonus Track","length":120000}}
			]
		}
	]
}`

const releaseListJSON = `{"releases":[{"id":"rel-001"}],"release-count":1}`

// newGroupContentServer builds a test server that handles both the
// release-group→release resolve step and the release detail fetch.
func newGroupContentServer(releaseDetailJSON string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("release-group") != "" {
			w.Write([]byte(releaseListJSON)) //nolint:errcheck
			return
		}
		w.Write([]byte(releaseDetailJSON)) //nolint:errcheck
	}))
}

func TestFetchGroupContent_Success(t *testing.T) {
	srv := newGroupContentServer(twoDiscRelease)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	items, total, err := a.FetchGroupContent(context.Background(), "rg-001", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 (sum of track-count across discs)", total)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	if items[0].RuntimeSecs != 180 {
		t.Errorf("items[0].RuntimeSecs = %d, want 180 (180000ms/1000)", items[0].RuntimeSecs)
	}
	// empty track title should fall back to recording title
	if items[2].Title != "Bonus Track" {
		t.Errorf("items[2].Title = %q, want Bonus Track (fallback to recording.title)", items[2].Title)
	}
	if items[2].ExternalID != "r-003" {
		t.Errorf("items[2].ExternalID = %q, want r-003", items[2].ExternalID)
	}
}

func TestFetchGroupContent_Pagination(t *testing.T) {
	srv := newGroupContentServer(twoDiscRelease)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	items, total, err := a.FetchGroupContent(context.Background(), "rg-001", 2, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1 (page 2 of perPage 1)", len(items))
	}
	if items[0].ExternalID != "r-002" {
		t.Errorf("items[0].ExternalID = %q, want r-002 (second track)", items[0].ExternalID)
	}
}

func TestFetchGroupContent_Empty(t *testing.T) {
	srv := newGroupContentServer(`{"media":[]}`)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	items, total, err := a.FetchGroupContent(context.Background(), "rg-empty", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
}

func TestFetchGroupContent_RoundTrip(t *testing.T) {
	const rgID = "rg-nirvana-nevermind"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "release-group"):
			w.Write([]byte(`{"release-group-count":1,"release-groups":[{"id":"rg-nirvana-nevermind","title":"Nevermind","first-release-date":"1991-09-24"}]}`)) //nolint:errcheck
		case r.URL.Query().Get("release-group") != "":
			w.Write([]byte(releaseListJSON)) //nolint:errcheck
		default:
			w.Write([]byte(twoDiscRelease)) //nolint:errcheck
		}
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})

	groups, _, _, err := a.FetchEntryContent(context.Background(), "artist-mbid", 1, 10)
	if err != nil {
		t.Fatalf("FetchEntryContent: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("FetchEntryContent returned no groups")
	}
	if groups[0].ExternalID != rgID {
		t.Errorf("groups[0].ExternalID = %q, want %q", groups[0].ExternalID, rgID)
	}

	items, total, err := a.FetchGroupContent(context.Background(), groups[0].ExternalID, 1, 10)
	if err != nil {
		t.Fatalf("FetchGroupContent with release-group MBID: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(items) != 3 {
		t.Errorf("len(items) = %d, want 3", len(items))
	}
}
