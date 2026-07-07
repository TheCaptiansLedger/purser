package mbz_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── FetchReleaseGroupReleases unit tests ──────────────────────────────────────

const rgReleasesJSON = `{
	"releases": [
		{
			"id":      "rel-us-1980",
			"title":   "Hi Infidelity",
			"status":  "Official",
			"country": "US",
			"date":    "1980-11-01",
			"barcode": "074643374120",
			"label-info": [{"label":{"name":"Epic"},"catalog-number":"FE 36844"}],
			"media": [{"format":"Vinyl","track-count":10}]
		},
		{
			"id":      "rel-uk-1980",
			"title":   "Hi Infidelity",
			"status":  "Official",
			"country": "GB",
			"date":    "1980-11-21",
			"barcode": "",
			"label-info": [{"label":{"name":"Epic"},"catalog-number":"EPC 84700"}],
			"media": [{"format":"Vinyl","track-count":10}]
		},
		{
			"id":      "rel-promo",
			"title":   "Hi Infidelity (Promo)",
			"status":  "Promotional",
			"country": "US",
			"date":    "1980-10-01",
			"barcode": "",
			"label-info": [],
			"media": [{"format":"CD","track-count":10}]
		},
		{
			"id":      "rel-digital-2024",
			"title":   "Hi Infidelity",
			"status":  "Official",
			"country": "XW",
			"date":    "2024-01-01",
			"barcode": "074646161425",
			"label-info": [{"label":{"name":"Legacy"},"catalog-number":""}],
			"media": [{"format":"Digital Media","track-count":10}]
		},
		{
			"id":      "rel-nodateof",
			"title":   "Hi Infidelity",
			"status":  "Official",
			"country": "US",
			"date":    "",
			"barcode": "",
			"label-info": [],
			"media": [{"format":"CD","track-count":10},{"format":"CD","track-count":2}]
		}
	]
}`

func newRGReleasesServer(releasesJSON string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(releasesJSON)) //nolint:errcheck
	}))
}

func TestMBZRelease_ParsesAllEditions(t *testing.T) {
	srv := newRGReleasesServer(rgReleasesJSON)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	releases, err := a.FetchReleaseGroupReleases(context.Background(), "rg-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 5 {
		t.Fatalf("len(releases) = %d, want 5", len(releases))
	}

	us := releases[0]
	if us.MBID != "rel-us-1980" {
		t.Errorf("MBID = %q, want rel-us-1980", us.MBID)
	}
	if us.Title != "Hi Infidelity" {
		t.Errorf("Title = %q, want Hi Infidelity", us.Title)
	}
	if us.Country != "US" {
		t.Errorf("Country = %q, want US", us.Country)
	}
	if us.Date != "1980-11-01" {
		t.Errorf("Date = %q, want 1980-11-01", us.Date)
	}
	if us.Barcode != "074643374120" {
		t.Errorf("Barcode = %q, want 074643374120", us.Barcode)
	}
	if us.Label != "Epic" {
		t.Errorf("Label = %q, want Epic", us.Label)
	}
	if us.CatalogNumber != "FE 36844" {
		t.Errorf("CatalogNumber = %q, want FE 36844", us.CatalogNumber)
	}
	if us.Format != "Vinyl" {
		t.Errorf("Format = %q, want Vinyl", us.Format)
	}
	if us.MediumCount != 1 {
		t.Errorf("MediumCount = %d, want 1", us.MediumCount)
	}
	if us.TrackCount != 10 {
		t.Errorf("TrackCount = %d, want 10", us.TrackCount)
	}

	// Two-medium release: track count must be the sum across all media.
	noDate := releases[4]
	if noDate.TrackCount != 12 {
		t.Errorf("noDate.TrackCount = %d, want 12 (10+2 across two media)", noDate.TrackCount)
	}
	if noDate.MediumCount != 2 {
		t.Errorf("noDate.MediumCount = %d, want 2", noDate.MediumCount)
	}
}

func TestMBZRelease_IsDefaultEarliestOfficial(t *testing.T) {
	srv := newRGReleasesServer(rgReleasesJSON)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	releases, err := a.FetchReleaseGroupReleases(context.Background(), "rg-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := 0
	for _, r := range releases {
		if r.IsDefault {
			defaults++
			if r.MBID != "rel-us-1980" {
				t.Errorf("IsDefault=true on %q, want rel-us-1980 (earliest Official)", r.MBID)
			}
		}
	}
	if defaults != 1 {
		t.Errorf("exactly one release must have IsDefault=true, got %d", defaults)
	}

	// Promotional release must never be default.
	for _, r := range releases {
		if r.MBID == "rel-promo" && r.IsDefault {
			t.Error("Promotional release must not be IsDefault")
		}
	}
}

// ── GetReleaseByBarcode unit tests ────────────────────────────────────────────

const barcodeSearchJSON = `{
	"count": 1,
	"releases": [
		{
			"id":      "1e639bf3-6b4c-4e1a-9d15-c61511804c8f",
			"title":   "Hi Infidelity",
			"status":  "Official",
			"country": "XW",
			"date":    "2024-01-01",
			"barcode": "074646161425",
			"label-info": [{"label":{"name":"Legacy"},"catalog-number":""}],
			"media": [{"format":"Digital Media","track-count":10}]
		}
	]
}`

func newBarcodeSearchServer(json string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(json)) //nolint:errcheck
	}))
}

func TestMBZRelease_BarcodeReturnsRelease(t *testing.T) {
	srv := newBarcodeSearchServer(barcodeSearchJSON)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	rel, err := a.GetReleaseByBarcode(context.Background(), "074646161425")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected a release, got nil")
	}
	if rel.MBID != "1e639bf3-6b4c-4e1a-9d15-c61511804c8f" {
		t.Errorf("MBID = %q, want 1e639bf3-6b4c-4e1a-9d15-c61511804c8f", rel.MBID)
	}
	if rel.Barcode != "074646161425" {
		t.Errorf("Barcode = %q, want 074646161425", rel.Barcode)
	}
	if rel.Format != "Digital Media" {
		t.Errorf("Format = %q, want Digital Media", rel.Format)
	}
	if rel.TrackCount != 10 {
		t.Errorf("TrackCount = %d, want 10", rel.TrackCount)
	}
}

func TestMBZRelease_BarcodeNoHit(t *testing.T) {
	srv := newBarcodeSearchServer(`{"count":0,"releases":[]}`)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	_, err := a.GetReleaseByBarcode(context.Background(), "000000000000")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ports.ErrNotFound for empty search result, got %v", err)
	}
}

// ── LookupISRC unit tests ─────────────────────────────────────────────────────

const isrcResponseJSON = `{
	"isrc": "USEE18000007",
	"recordings": [
		{
			"id": "rec-001",
			"releases": [
				{"id": "rel-001", "release-group": {"id": "rg-hi-inf", "title": "Hi Infidelity", "first-release-date": "1980-11-01", "primary-type": "Album"}},
				{"id": "rel-002", "release-group": {"id": "rg-hi-inf", "title": "Hi Infidelity", "first-release-date": "1980-11-01", "primary-type": "Album"}}
			]
		},
		{
			"id": "rec-002",
			"releases": [
				{"id": "rel-003", "release-group": {"id": "rg-hi-inf", "title": "Hi Infidelity", "first-release-date": "1980-11-01", "primary-type": "Album"}}
			]
		}
	]
}`

func newISRCServer(json string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(json)) //nolint:errcheck
	}))
}

func TestMBZISRC_ReturnsReleaseGroupMBID(t *testing.T) {
	srv := newISRCServer(isrcResponseJSON)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	rgMBID, err := a.LookupISRC(context.Background(), "USEE18000007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rgMBID != "rg-hi-inf" {
		t.Errorf("rgMBID = %q, want rg-hi-inf", rgMBID)
	}
}

func TestMBZISRC_NoReleaseGroup(t *testing.T) {
	srv := newISRCServer(`{"isrc":"USXX0000000","recordings":[{"id":"rec-001","releases":[{"id":"rel-001","release-group":{"id":"","title":"","first-release-date":"","primary-type":""}}]}]}`)
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	rgMBID, err := a.LookupISRC(context.Background(), "USXX0000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rgMBID != "" {
		t.Errorf("expected empty rgMBID when release-group id is empty, got %q", rgMBID)
	}
}

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

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	items, total, err := a.FetchGroupContent(context.Background(), domain.ContentTypeMusic, "rg-001", 1, 10)
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

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	items, total, err := a.FetchGroupContent(context.Background(), domain.ContentTypeMusic, "rg-001", 2, 1)
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

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	items, total, err := a.FetchGroupContent(context.Background(), domain.ContentTypeMusic, "rg-empty", 1, 10)
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
		case r.URL.Query().Get("artist") != "":
			// FetchEntryContent: Official release browse with embedded release-group.
			w.Write([]byte(`{"release-count":1,"releases":[{"release-group":{"id":"rg-nirvana-nevermind","title":"Nevermind","first-release-date":"1991-09-24","primary-type":"Album"}}]}`)) //nolint:errcheck
		case r.URL.Query().Get("release-group") != "":
			// resolveToReleaseMBID: list releases for the release-group.
			w.Write([]byte(releaseListJSON)) //nolint:errcheck
		default:
			// FetchGroupContent detail: track recordings for a specific release.
			w.Write([]byte(twoDiscRelease)) //nolint:errcheck
		}
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)

	groups, _, _, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "artist-mbid", 1, 10)
	if err != nil {
		t.Fatalf("FetchEntryContent: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("FetchEntryContent returned no groups")
	}
	if groups[0].ExternalID != rgID {
		t.Errorf("groups[0].ExternalID = %q, want %q", groups[0].ExternalID, rgID)
	}

	items, total, err := a.FetchGroupContent(context.Background(), domain.ContentTypeMusic, groups[0].ExternalID, 1, 10)
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
