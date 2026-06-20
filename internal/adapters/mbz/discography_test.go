package mbz_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"strings"
	"testing"
)

func TestFetchEntryContent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"release-group-count":3,"release-groups":[` +
			`{"id":"abc-123","title":"Nevermind","first-release-date":"1991-09-24","primary-type":"Album"},` +
			`{"id":"def-456","title":"In Utero","first-release-date":"1993","primary-type":"Album"},` +
			`{"id":"ghi-789","title":"Live at Hollywood","first-release-date":"2000","primary-type":"Album","secondary-types":["Live"]}` +
			`]}`)) //nolint:errcheck
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
		w.Write([]byte(`{"release-group-count": 0, "release-groups": []}`)) //nolint:errcheck
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

func TestFetchEntryContent_PaginationOffset(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"release-group-count": 0, "release-groups": []}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, _, _, _ = a.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "some-mbid", 3, 10)
	if !strings.Contains(query, "offset=20") {
		t.Errorf("expected offset=20 for page=3 perPage=10, got query: %s", query)
	}
}
