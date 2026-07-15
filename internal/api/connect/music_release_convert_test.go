package apiconnect

import (
	"purser/internal/domain/music"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

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

func TestApplyMusicReleaseFieldMask(t *testing.T) {
	addedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	existing := &music.Release{
		ID: "r1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Original", Country: "Original Country",
		Status: music.ReleaseStatusStub, AddedAt: &addedAt, UpdatedAt: &addedAt,
	}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyMusicReleaseFieldMask(existing, &musicv1.Release{Id: "r1", Title: "New", Status: musicv1.ReleaseStatus_RELEASE_STATUS_STUB}, nil)
		if got.Title != "New" {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
	})

	t.Run("id, added_at, and updated_at are never mask-updatable", func(t *testing.T) {
		got := applyMusicReleaseFieldMask(existing, &musicv1.Release{Id: "should-be-ignored", Title: "New", Status: musicv1.ReleaseStatus_RELEASE_STATUS_STUB}, nil)
		if got.ID != "r1" {
			t.Fatalf("applyMusicReleaseFieldMask changed ID: got %q, want %q", got.ID, "r1")
		}
		if got.AddedAt == nil || !got.AddedAt.Equal(addedAt) {
			t.Fatalf("applyMusicReleaseFieldMask changed AddedAt: got %v", got.AddedAt)
		}
		if got.UpdatedAt == nil || !got.UpdatedAt.Equal(addedAt) {
			t.Fatalf("applyMusicReleaseFieldMask changed UpdatedAt: got %v", got.UpdatedAt)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		date := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		full := &musicv1.Release{
			GroupId: "group2", LibraryEntryId: "entry2", Title: "N", Country: "GB", Label: "L", CatalogNumber: "C2",
			Barcode: "B2", Format: "Vinyl", MediumCount: 2, TrackCount: 20, IsDefault: true, Monitored: true,
			Status: musicv1.ReleaseStatus_RELEASE_STATUS_IMPORTED, Mbid: "mbid2",
		}
		full.Date = musicReleaseToProto(&music.Release{Date: &date}).Date
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"group_id", "library_entry_id", "title", "country", "date", "label", "catalog_number", "barcode",
			"format", "medium_count", "track_count", "is_default", "monitored", "status", "mbid", "unrecognized",
		}}
		got := applyMusicReleaseFieldMask(existing, full, mask)
		if got.GroupID != "group2" || got.LibraryEntryID != "entry2" || got.Title != "N" || got.Country != "GB" ||
			got.Label != "L" || got.CatalogNumber != "C2" || got.Barcode != "B2" || got.Format != "Vinyl" ||
			got.MediumCount != 2 || got.TrackCount != 20 || !got.IsDefault || !got.Monitored ||
			got.Status != music.ReleaseStatusImported || got.MBID != "mbid2" {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
		if got.Date == nil || !got.Date.Equal(date) {
			t.Fatalf("date path was not applied: got %v", got.Date)
		}
	})

	t.Run("unmasked field is untouched", func(t *testing.T) {
		mask := &fieldmaskpb.FieldMask{Paths: []string{"title"}}
		got := applyMusicReleaseFieldMask(existing, &musicv1.Release{Id: "r1", Title: "New Title", Country: "Should be ignored"}, mask)
		if got.Title != "New Title" {
			t.Fatalf("applyMusicReleaseFieldMask did not apply Title: got %+v", got)
		}
		if got.Country != "Original Country" {
			t.Fatalf("applyMusicReleaseFieldMask touched unmasked Country: got %q", got.Country)
		}
	})
}
