package apiconnect

import (
	"purser/internal/domain/music"
	"testing"
	"time"

	musicv1 "purser/gen/go/purser/music/v1"
)

func TestReleaseStatusRoundTrip(t *testing.T) {
	tests := []music.ReleaseStatus{music.ReleaseStatusStub, music.ReleaseStatusPartial, music.ReleaseStatusImported}
	for _, s := range tests {
		if got := releaseStatusFromProto(releaseStatusToProto(s)); got != s {
			t.Fatalf("round trip changed status: got %q, want %q", got, s)
		}
	}
	if got := releaseStatusToProto(music.ReleaseStatus("unknown")); got != musicv1.ReleaseStatus_RELEASE_STATUS_UNSPECIFIED {
		t.Fatalf("releaseStatusToProto(unknown) = %v, want RELEASE_STATUS_UNSPECIFIED", got)
	}
	if got := releaseStatusFromProto(musicv1.ReleaseStatus_RELEASE_STATUS_UNSPECIFIED); got != music.ReleaseStatus("") {
		t.Fatalf("releaseStatusFromProto(UNSPECIFIED) = %q, want empty", got)
	}
}

func TestMusicReleaseRoundTrip(t *testing.T) {
	if musicReleaseToProto(nil) != nil {
		t.Fatal("musicReleaseToProto(nil) did not return nil")
	}
	if musicReleaseFromProto(nil) == nil {
		t.Fatal("musicReleaseFromProto(nil) returned nil, want a zero-value Release")
	}

	date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	addedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	r := &music.Release{
		ID: "r1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Hi Infidelity (2024 Remaster)",
		Country: "US", Date: &date, Label: "CBS", CatalogNumber: "FZ 38079", Barcode: "07464538792",
		Format: "CD", MediumCount: 1, TrackCount: 10, IsDefault: true, Monitored: true,
		Status: music.ReleaseStatusImported, MBID: "89ad4ac3-39f7-470e-963a-56509c546377",
		AddedAt: &addedAt, UpdatedAt: &addedAt,
	}

	got := musicReleaseFromProto(musicReleaseToProto(r))
	if got.ID != r.ID || got.GroupID != r.GroupID || got.LibraryEntryID != r.LibraryEntryID || got.Title != r.Title ||
		got.Country != r.Country || got.Label != r.Label || got.CatalogNumber != r.CatalogNumber || got.Barcode != r.Barcode ||
		got.Format != r.Format || got.MediumCount != r.MediumCount || got.TrackCount != r.TrackCount ||
		got.IsDefault != r.IsDefault || got.Monitored != r.Monitored || got.Status != r.Status || got.MBID != r.MBID {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
	if got.Date == nil || !got.Date.Equal(date) {
		t.Fatalf("round trip changed Date: got %v", got.Date)
	}
	if got.AddedAt == nil || !got.AddedAt.Equal(addedAt) {
		t.Fatalf("round trip changed AddedAt: got %v", got.AddedAt)
	}
	if got.UpdatedAt == nil || !got.UpdatedAt.Equal(addedAt) {
		t.Fatalf("round trip changed UpdatedAt: got %v", got.UpdatedAt)
	}
}
