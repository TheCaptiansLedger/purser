package mbz_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"strings"
	"testing"
)

func TestSearchItems_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":1,"recordings":[{"id":"rec-001","title":"Smells Like Teen Spirit","length":301000,"releases":[{"artist-credit":[{"name":"Nirvana","artist":{"id":"art-001","name":"Nirvana"}}]}]}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	items, err := a.SearchItems(context.Background(), domain.ContentTypeMusic, "Smells Like Teen Spirit", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.Title != "Smells Like Teen Spirit" {
		t.Errorf("Title = %q, want Smells Like Teen Spirit", item.Title)
	}
	if item.RuntimeSecs != 301 {
		t.Errorf("RuntimeSecs = %d, want 301 (301000ms / 1000)", item.RuntimeSecs)
	}
	if item.Studio == nil {
		t.Fatal("Studio should be populated from artist-credit")
	}
	if item.Studio.Name != "Nirvana" {
		t.Errorf("Studio.Name = %q, want Nirvana", item.Studio.Name)
	}
}

func TestSearchItems_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"recordings":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	items, err := a.SearchItems(context.Background(), domain.ContentTypeMusic, "nothing", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
}

// recordingWithTwoReleases returns a fake MBZ recording JSON with two releases
// belonging to the same release group. Used across multiple fetch tests.
const recordingWithTwoReleases = `{
  "id": "rec-001",
  "title": "Take It on the Run",
  "length": 238000,
  "artist-credit": [{"name": "REO Speedwagon", "artist": {"id": "art-001", "name": "REO Speedwagon"}}],
  "releases": [
    {
      "id": "rel-001",
      "title": "Hi Infidelity",
      "release-group": {"id": "rg-001", "title": "Hi Infidelity"},
      "date": "1980-11-21",
      "country": "US",
      "barcode": "07464364342",
      "label-info": [{"label": {"name": "Epic"}, "catalog-number": "FE 36844"}]
    },
    {
      "id": "rel-002",
      "title": "Hi Infidelity (2024 Remaster)",
      "release-group": {"id": "rg-001", "title": "Hi Infidelity"},
      "date": "2024-03-01",
      "country": "US",
      "barcode": "196588821325",
      "label-info": [{"label": {"name": "Sony"}, "catalog-number": "19658882132"}]
    }
  ]
}`

// TestToExternalRecording_GroupExternalID verifies the core invariant of Task 4:
// GroupExternalID must be the release group MBID, never the release MBID or the
// recording MBID. Without this fix, scanned files can never match imported albums
// because the two ID namespaces don't overlap.
func TestToExternalRecording_GroupExternalID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(recordingWithTwoReleases)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	item, err := a.FetchRecordingByID(context.Background(), "rec-001", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.ExternalID != "rec-001" {
		t.Errorf("ExternalID = %q, want rec-001 (recording MBID)", item.ExternalID)
	}
	if item.GroupExternalID != "rg-001" {
		t.Errorf("GroupExternalID = %q, want rg-001 (release group MBID)", item.GroupExternalID)
	}
	// T4 invariant: GroupExternalID must never equal ExternalID.
	if item.GroupExternalID == item.ExternalID {
		t.Errorf("GroupExternalID == ExternalID (%q); release group MBID must differ from recording MBID", item.ExternalID)
	}
	// ReleaseDetail carries the specific release MBID; GroupExternalID must differ from it too.
	if item.ReleaseDetail == nil {
		t.Fatal("ReleaseDetail is nil; expected it to be populated for the selected release")
	}
	if item.ReleaseDetail.ReleaseMBID == item.GroupExternalID {
		t.Errorf("ReleaseDetail.ReleaseMBID (%q) == GroupExternalID (%q); release MBID must differ from release group MBID",
			item.ReleaseDetail.ReleaseMBID, item.GroupExternalID)
	}
	if item.ReleaseDetail.ReleaseMBID != "rel-001" {
		t.Errorf("ReleaseDetail.ReleaseMBID = %q, want rel-001 (first release selected when no hint)", item.ReleaseDetail.ReleaseMBID)
	}
}

// TestPickRelease_AlbumHint verifies that FetchRecordingByID selects the release
// whose title matches the albumHint substring, not always the first release.
func TestPickRelease_AlbumHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(recordingWithTwoReleases)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	item, err := a.FetchRecordingByID(context.Background(), "rec-001", "2024 Remaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.ReleaseDetail == nil {
		t.Fatal("ReleaseDetail is nil")
	}
	if item.ReleaseDetail.ReleaseMBID != "rel-002" {
		t.Errorf("ReleaseMBID = %q, want rel-002 (the remaster matches the hint)", item.ReleaseDetail.ReleaseMBID)
	}
	if item.GroupExternalID != "rg-001" {
		t.Errorf("GroupExternalID = %q, want rg-001; album hint must not change which release group is selected", item.GroupExternalID)
	}
}

// TestToExternalRecording_MissingReleaseGroup verifies that a release with no
// release-group ID results in a graceful return (no panic), GroupExternalID left
// empty, and a WARN log emitted.
func TestToExternalRecording_MissingReleaseGroup(t *testing.T) {
	const body = `{
	  "id": "rec-002",
	  "title": "Some Track",
	  "length": 180000,
	  "releases": [{"id": "rel-999", "title": "Some Album", "release-group": {}}]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body)) //nolint:errcheck
	}))
	defer srv.Close()

	var logBuf strings.Builder
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	item, err := a.FetchRecordingByID(context.Background(), "rec-002", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.GroupExternalID != "" {
		t.Errorf("GroupExternalID = %q, want empty when release-group is missing", item.GroupExternalID)
	}
	if item.ReleaseDetail == nil {
		t.Error("ReleaseDetail is nil; release was selected so detail should still be populated")
	}
	if !strings.Contains(logBuf.String(), "mbz: release has no release-group") {
		t.Errorf("expected WARN log containing 'mbz: release has no release-group'; got: %s", logBuf.String())
	}
}

// TestToExternalRecording_ReleaseDetail verifies that label, catalog, country,
// barcode, and date are all correctly extracted from the MBZ response.
func TestToExternalRecording_ReleaseDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(recordingWithTwoReleases)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	item, err := a.FetchRecordingByID(context.Background(), "rec-001", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := item.ReleaseDetail
	if d == nil {
		t.Fatal("ReleaseDetail is nil")
	}

	tests := []struct {
		field string
		got   string
		want  string
	}{
		{"ReleaseTitle", d.ReleaseTitle, "Hi Infidelity"},
		{"ReleaseDate", d.ReleaseDate, "1980-11-21"},
		{"ReleaseLabel", d.ReleaseLabel, "Epic"},
		{"ReleaseCountry", d.ReleaseCountry, "US"},
		{"ReleaseCatalog", d.ReleaseCatalog, "FE 36844"},
		{"ReleaseBarcode", d.ReleaseBarcode, "07464364342"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("ReleaseDetail.%s = %q, want %q", tt.field, tt.got, tt.want)
		}
	}
	if item.GroupTitle != "Hi Infidelity" {
		t.Errorf("GroupTitle = %q, want Hi Infidelity (release group title, not edition-qualified release title)", item.GroupTitle)
	}
}
