package apiconnect

import (
	"purser/internal/domain"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestItemStatusRoundTrip(t *testing.T) {
	cases := []struct {
		domainVal domain.ItemStatus
		protoVal  v1.ItemStatus
	}{
		{domain.ItemStatusWanted, v1.ItemStatus_ITEM_STATUS_WANTED},
		{domain.ItemStatusGrabbed, v1.ItemStatus_ITEM_STATUS_GRABBED},
		{domain.ItemStatusDownloading, v1.ItemStatus_ITEM_STATUS_DOWNLOADING},
		{domain.ItemStatusImported, v1.ItemStatus_ITEM_STATUS_IMPORTED},
		{domain.ItemStatusMissing, v1.ItemStatus_ITEM_STATUS_MISSING},
		{domain.ItemStatusSkipped, v1.ItemStatus_ITEM_STATUS_SKIPPED},
	}
	for _, c := range cases {
		if got := itemStatusToProto(c.domainVal); got != c.protoVal {
			t.Errorf("itemStatusToProto(%v) = %v, want %v", c.domainVal, got, c.protoVal)
		}
		if got := itemStatusFromProto(c.protoVal); got != c.domainVal {
			t.Errorf("itemStatusFromProto(%v) = %v, want %v", c.protoVal, got, c.domainVal)
		}
	}
	if got := itemStatusFromProto(v1.ItemStatus_ITEM_STATUS_UNSPECIFIED); got != domain.ItemStatus("") {
		t.Errorf("itemStatusFromProto(UNSPECIFIED) = %q, want zero value", got)
	}
	if got := itemStatusToProto(domain.ItemStatus("bogus")); got != v1.ItemStatus_ITEM_STATUS_UNSPECIFIED {
		t.Errorf("itemStatusToProto(bogus) = %v, want ITEM_STATUS_UNSPECIFIED", got)
	}
}

func TestItemRoundTrip(t *testing.T) {
	if itemToProto(nil) != nil {
		t.Fatal("itemToProto(nil) did not return nil")
	}
	if itemFromProto(nil) == nil {
		t.Fatal("itemFromProto(nil) returned nil, want a zero-value Item")
	}

	date := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	i := &domain.Item{
		ID: "i1", ContentType: domain.ContentTypeAdult, LibraryEntryID: "e1", GroupID: "g1",
		Title: "T", Overview: "O", Date: &date, Sequence: "1", RuntimeSeconds: 120,
		Monitored: true, Status: domain.ItemStatusImported, Metadata: map[string]any{"k": "v"},
	}
	got := itemFromProto(itemToProto(i))
	if got.ID != i.ID || got.ContentType != i.ContentType || got.Title != i.Title || got.Status != i.Status {
		t.Fatalf("round trip changed identity fields: got %+v", got)
	}
	if got.Date == nil || !got.Date.Equal(date) {
		t.Fatalf("round trip changed Date: got %v", got.Date)
	}
	if got.Metadata["k"] != "v" {
		t.Fatalf("round trip changed Metadata: got %v", got.Metadata)
	}

	t.Run("nil date stays nil", func(t *testing.T) {
		noDate := &domain.Item{ID: "i2", Title: "No Date"}
		got := itemFromProto(itemToProto(noDate))
		if got.Date != nil {
			t.Fatalf("round trip invented a Date: %v", got.Date)
		}
	})
}

func TestApplyItemFieldMask(t *testing.T) {
	existing := &domain.Item{ID: "i1", Title: "Original", Overview: "Original Overview", Status: domain.ItemStatusWanted}

	t.Run("nil mask replaces every mutable field", func(t *testing.T) {
		got := applyItemFieldMask(existing, &v1.Item{Id: "i1", Title: "New"}, nil)
		if got.Title != "New" {
			t.Fatalf("nil mask did not fully replace: got %+v", got)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &v1.Item{
			ContentType: "movie", LibraryEntryId: "e2", GroupId: "g2", Title: "N", Overview: "O2",
			Sequence: "2", RuntimeSeconds: 60, Monitored: true, Status: v1.ItemStatus_ITEM_STATUS_MISSING,
			Metadata: metadataToProto(map[string]any{"k": "v"}),
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"content_type", "library_entry_id", "group_id", "title", "overview", "date",
			"sequence", "runtime_seconds", "monitored", "status", "metadata", "unrecognized",
		}}
		got := applyItemFieldMask(existing, full, mask)
		if got.ContentType != "movie" || got.LibraryEntryID != "e2" || got.GroupID != "g2" || got.Title != "N" ||
			got.Overview != "O2" || got.Sequence != "2" || got.RuntimeSeconds != 60 || !got.Monitored ||
			got.Status != domain.ItemStatusMissing || got.Metadata["k"] != "v" {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
