package apiconnect

import (
	"purser/internal/domain"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestGroupRoundTrip(t *testing.T) {
	if groupToProto(nil) != nil {
		t.Fatal("groupToProto(nil) did not return nil")
	}
	if groupFromProto(nil) == nil {
		t.Fatal("groupFromProto(nil) returned nil, want a zero-value Group")
	}

	g := &domain.Group{
		ID: "g1", LibraryEntryID: "e1", Title: "T", SortName: "T, The", Number: "1",
		Year: 2024, Overview: "O", Monitored: true, MonitorMode: domain.MonitorModeAll,
		Metadata: map[string]any{"k": "v"},
	}
	got := groupFromProto(groupToProto(g))
	if got.ID != g.ID || got.LibraryEntryID != g.LibraryEntryID || got.Title != g.Title ||
		got.Number != g.Number || got.Year != g.Year || got.Metadata["k"] != "v" {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
}

func TestApplyGroupFieldMask(t *testing.T) {
	existing := &domain.Group{ID: "g1", Title: "Original", Overview: "Original Overview", MonitorMode: domain.MonitorModeNone}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyGroupFieldMask(existing, &v1.Group{Id: "g1", Title: "New"}, nil)
		if got.Title != "New" {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &v1.Group{
			LibraryEntryId: "e2", Title: "N", SortName: "S", Number: "2", Year: 2025,
			Overview: "O2", Monitored: true, MonitorMode: v1.MonitorMode_MONITOR_MODE_LATEST,
			Metadata: metadataToProto(map[string]any{"k": "v"}),
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"library_entry_id", "title", "sort_name", "number", "year",
			"overview", "monitored", "monitor_mode", "metadata", "unrecognized",
		}}
		got := applyGroupFieldMask(existing, full, mask)
		if got.LibraryEntryID != "e2" || got.Title != "N" || got.SortName != "S" || got.Number != "2" ||
			got.Year != 2025 || got.Overview != "O2" || !got.Monitored || got.MonitorMode != domain.MonitorModeLatest ||
			got.Metadata["k"] != "v" {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
