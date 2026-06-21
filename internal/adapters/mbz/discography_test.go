package mbz_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"strings"
	"sync/atomic"
	"testing"
)

// releaseBrowseFixture simulates a single-page Official release browse response
// with three distinct release-groups.
const releaseBrowseFixture = `{
	"release-count": 3,
	"releases": [
		{"release-group": {"id":"rg-001","title":"Nevermind","first-release-date":"1991-09-24","primary-type":"Album","secondary-types":[]}},
		{"release-group": {"id":"rg-002","title":"In Utero","first-release-date":"1993","primary-type":"Album","secondary-types":[]}},
		{"release-group": {"id":"rg-003","title":"Live at Hollywood","first-release-date":"2000","primary-type":"Album","secondary-types":["Live"]}}
	]
}`

func TestFetchEntryContent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(releaseBrowseFixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	groups, items, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Error("items should be nil for music hierarchy")
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(groups) != 3 {
		t.Fatalf("len(groups) = %d, want 3", len(groups))
	}
	if groups[0].Title != "Nevermind" {
		t.Errorf("groups[0].Title = %q, want Nevermind", groups[0].Title)
	}
	if groups[0].Year != 1991 {
		t.Errorf("groups[0].Year = %d, want 1991 (from first-release-date)", groups[0].Year)
	}
	if groups[1].Year != 1993 {
		t.Errorf("groups[1].Year = %d, want 1993 (year parsed from partial date)", groups[1].Year)
	}
	if groups[0].PrimaryType != "Album" {
		t.Errorf("groups[0].PrimaryType = %q, want Album", groups[0].PrimaryType)
	}
	if len(groups[0].SecondaryTypes) != 0 {
		t.Errorf("groups[0].SecondaryTypes = %v, want empty", groups[0].SecondaryTypes)
	}
	if len(groups[2].SecondaryTypes) != 1 || groups[2].SecondaryTypes[0] != "Live" {
		t.Errorf("groups[2].SecondaryTypes = %v, want [Live]", groups[2].SecondaryTypes)
	}
}

func TestFetchEntryContent_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"release-count": 0, "releases": []}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	groups, _, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(groups) != 0 {
		t.Errorf("len(groups) = %d, want 0", len(groups))
	}
}

func TestFetchEntryContent_PaginationSlice(t *testing.T) {
	// Five unique release-groups; page 2 with perPage=2 must return indices 2 and 3.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"release-count":5,"releases":[
			{"release-group":{"id":"rg-1","title":"A","first-release-date":"2001","primary-type":"Album"}},
			{"release-group":{"id":"rg-2","title":"B","first-release-date":"2002","primary-type":"Album"}},
			{"release-group":{"id":"rg-3","title":"C","first-release-date":"2003","primary-type":"Album"}},
			{"release-group":{"id":"rg-4","title":"D","first-release-date":"2004","primary-type":"Album"}},
			{"release-group":{"id":"rg-5","title":"E","first-release-date":"2005","primary-type":"Album"}}
		]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	groups, _, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].Title != "C" {
		t.Errorf("groups[0].Title = %q, want C (page 2, perPage 2)", groups[0].Title)
	}
	if groups[1].Title != "D" {
		t.Errorf("groups[1].Title = %q, want D", groups[1].Title)
	}
}

func TestFetchEntryContent_OfficialStatusOnReleaseEndpoint(t *testing.T) {
	// MusicBrainz release browse requires lowercase "official" — the capitalized
	// form "Official" returns HTTP 400 ("Official is not a recognized release status").
	// Verified against the live API before this test was written.
	var requestURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"release-count": 0, "releases": []}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, _, _, _ = a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 10)

	if !strings.HasPrefix(requestURL, "/release?") {
		t.Errorf("expected request to /release endpoint, got: %s", requestURL)
	}
	if !strings.Contains(requestURL, "status=official") {
		t.Errorf("MBZ release browse requires lowercase status=official; got: %s", requestURL)
	}
	if strings.Contains(requestURL, "status=Official") {
		t.Errorf("status=Official (capitalized) returns HTTP 400 from MBZ; got: %s", requestURL)
	}
	if !strings.Contains(requestURL, "inc=release-groups") {
		t.Errorf("expected inc=release-groups in query to embed release-group data; got: %s", requestURL)
	}
}

func TestFetchEntryContent_DeduplicatesReleaseGroups(t *testing.T) {
	// Three Official releases share one release-group (e.g. US, UK, and remaster
	// editions of the same album). The adapter must return exactly one ExternalGroup.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"release-count":4,"releases":[
			{"release-group":{"id":"rg-abbey","title":"Abbey Road","first-release-date":"1969-09-26","primary-type":"Album"}},
			{"release-group":{"id":"rg-abbey","title":"Abbey Road","first-release-date":"1969-09-26","primary-type":"Album"}},
			{"release-group":{"id":"rg-abbey","title":"Abbey Road","first-release-date":"1969-09-26","primary-type":"Album"}},
			{"release-group":{"id":"rg-white","title":"The Beatles","first-release-date":"1968-11-22","primary-type":"Album"}}
		]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	groups, _, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2 (three editions of Abbey Road deduplicated to one)", total)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].ExternalID != "rg-abbey" {
		t.Errorf("groups[0].ExternalID = %q, want rg-abbey", groups[0].ExternalID)
	}
	if groups[1].ExternalID != "rg-white" {
		t.Errorf("groups[1].ExternalID = %q, want rg-white", groups[1].ExternalID)
	}
}

func TestFetchEntryContent_MultiPageReleases(t *testing.T) {
	// When release-count exceeds one page the adapter must issue follow-up
	// requests with increasing offsets until all releases are collected.
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			// First page: signals 150 total releases so a second fetch is required.
			w.Write([]byte(`{"release-count":150,"releases":[
				{"release-group":{"id":"rg-1","title":"Album One","first-release-date":"1990","primary-type":"Album"}}
			]}`)) //nolint:errcheck
		default:
			// Subsequent page: remaining releases, new release-group.
			w.Write([]byte(`{"release-count":150,"releases":[
				{"release-group":{"id":"rg-2","title":"Album Two","first-release-date":"1995","primary-type":"Album"}}
			]}`)) //nolint:errcheck
		}
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	groups, _, total, err := a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount.Load() < 2 {
		t.Errorf("expected at least 2 HTTP requests for multi-page release browse, got %d", requestCount.Load())
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
}
