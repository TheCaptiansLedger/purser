package apiconnect

import (
	"purser/internal/domain"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestMetadataRoundTrip(t *testing.T) {
	if metadataToProto(nil) != nil {
		t.Fatal("metadataToProto(nil) did not return nil")
	}
	if metadataFromProto(nil) != nil {
		t.Fatal("metadataFromProto(nil) did not return nil")
	}

	m := map[string]any{"key": "value", "count": float64(3)}
	got := metadataFromProto(metadataToProto(m))
	if got["key"] != "value" || got["count"] != float64(3) {
		t.Fatalf("metadata round trip = %+v, want %+v", got, m)
	}
}

func TestLibraryEntryRoundTrip(t *testing.T) {
	if libraryEntryToProto(nil) != nil {
		t.Fatal("libraryEntryToProto(nil) did not return nil")
	}
	if libraryEntryFromProto(nil) == nil {
		t.Fatal("libraryEntryFromProto(nil) returned nil, want a zero-value LibraryEntry")
	}

	e := &domain.LibraryEntry{
		ID: "e1", ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "Studio", SortName: "Studio, The", Overview: "O", ParentID: "p1",
		Monitored: true, MonitorMode: domain.MonitorModeAll, Status: "active",
		QualityProfileID: "qp1", MetadataProfileID: "mp1", Path: "/media",
		Metadata: map[string]any{"k": "v"},
	}
	got := libraryEntryFromProto(libraryEntryToProto(e))
	if got.ID != e.ID || got.ContentType != e.ContentType || got.Kind != e.Kind || got.Name != e.Name {
		t.Fatalf("round trip changed identity fields: got %+v", got)
	}
	if got.Status != e.Status || got.ParentID != e.ParentID || got.Path != e.Path {
		t.Fatalf("round trip changed Status/ParentID/Path: got %+v", got)
	}
	if got.Metadata["k"] != "v" {
		t.Fatalf("round trip changed Metadata: got %v", got.Metadata)
	}
}

func TestApplyLibraryEntryFieldMask(t *testing.T) {
	existing := &domain.LibraryEntry{ID: "e1", Name: "Original", Overview: "Original Overview", MonitorMode: domain.MonitorModeNone}
	incoming := &v1.LibraryEntry{Id: "e1", Name: "New", Overview: "New Overview"}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyLibraryEntryFieldMask(existing, incoming, nil)
		if got.Name != "New" || got.Overview != "New Overview" {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
	})

	t.Run("named path applies only that field", func(t *testing.T) {
		got := applyLibraryEntryFieldMask(existing, incoming, &fieldmaskpb.FieldMask{Paths: []string{"name"}})
		if got.Name != "New" || got.Overview != "Original Overview" {
			t.Fatalf("masked update touched the wrong fields: got %+v", got)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &v1.LibraryEntry{
			ContentType: "movie", Kind: "movie", Name: "N", SortName: "S", Overview: "O",
			ParentId: "p", Monitored: true, MonitorMode: v1.MonitorMode_MONITOR_MODE_LATEST,
			Status: "st", QualityProfileId: "qp", MetadataProfileId: "mp", Path: "path",
			Metadata: metadataToProto(map[string]any{"k": "v"}),
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"content_type", "kind", "name", "sort_name", "overview", "parent_id",
			"monitored", "monitor_mode", "status", "quality_profile_id",
			"metadata_profile_id", "path", "metadata", "unrecognized",
		}}
		got := applyLibraryEntryFieldMask(existing, full, mask)
		if got.ContentType != "movie" || got.Kind != "movie" || got.Name != "N" || got.SortName != "S" ||
			got.Overview != "O" || got.ParentID != "p" || !got.Monitored || got.MonitorMode != domain.MonitorModeLatest ||
			got.Status != "st" || got.QualityProfileID != "qp" || got.MetadataProfileID != "mp" || got.Path != "path" ||
			got.Metadata["k"] != "v" {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
