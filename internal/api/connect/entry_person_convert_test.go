package apiconnect

import (
	"purser/internal/domain"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestEntryPersonRoundTrip(t *testing.T) {
	if entryPersonToProto(nil) != nil {
		t.Fatal("entryPersonToProto(nil) did not return nil")
	}
	if entryPersonFromProto(nil) == nil {
		t.Fatal("entryPersonFromProto(nil) returned nil, want a zero-value EntryPerson")
	}

	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	ep := &domain.EntryPerson{
		LibraryEntryID: "e1", PersonID: "p1", Role: "director",
		CreditedAs: "Credit", Character: "Char", StartDate: &start, EndDate: &end,
	}
	got := entryPersonFromProto(entryPersonToProto(ep))
	if got.LibraryEntryID != ep.LibraryEntryID || got.PersonID != ep.PersonID || got.Role != ep.Role ||
		got.CreditedAs != ep.CreditedAs || got.Character != ep.Character {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
	if got.StartDate == nil || !got.StartDate.Equal(start) || got.EndDate == nil || !got.EndDate.Equal(end) {
		t.Fatalf("round trip changed dates: got start=%v end=%v", got.StartDate, got.EndDate)
	}

	t.Run("nil dates stay nil", func(t *testing.T) {
		noDates := &domain.EntryPerson{LibraryEntryID: "e1", PersonID: "p1", Role: "director"}
		got := entryPersonFromProto(entryPersonToProto(noDates))
		if got.StartDate != nil || got.EndDate != nil {
			t.Fatalf("round trip invented dates: start=%v end=%v", got.StartDate, got.EndDate)
		}
	})
}

func TestApplyEntryPersonFieldMask(t *testing.T) {
	existing := &domain.EntryPerson{LibraryEntryID: "e1", PersonID: "p1", Role: "director", CreditedAs: "Original", Character: "Original Char"}

	t.Run("nil mask replaces every mutable field, key fields stay fixed", func(t *testing.T) {
		incoming := &v1.EntryPerson{LibraryEntryId: "different", PersonId: "different", Role: "different", CreditedAs: "New"}
		got := applyEntryPersonFieldMask(existing, incoming, nil)
		if got.CreditedAs != "New" {
			t.Fatalf("nil mask did not replace CreditedAs: got %+v", got)
		}
		if got.LibraryEntryID != "e1" || got.PersonID != "p1" || got.Role != "director" {
			t.Fatalf("mask changed the composite key fields: got %+v", got)
		}
	})

	t.Run("named path applies only that field", func(t *testing.T) {
		incoming := &v1.EntryPerson{CreditedAs: "New", Character: "Should be ignored"}
		got := applyEntryPersonFieldMask(existing, incoming, &fieldmaskpb.FieldMask{Paths: []string{"credited_as"}})
		if got.CreditedAs != "New" || got.Character != "Original Char" {
			t.Fatalf("masked update touched the wrong fields: got %+v", got)
		}
	})

	t.Run("start_date and end_date paths apply", func(t *testing.T) {
		start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		incoming := entryPersonToProto(&domain.EntryPerson{StartDate: &start})
		got := applyEntryPersonFieldMask(existing, incoming, &fieldmaskpb.FieldMask{Paths: []string{"start_date", "end_date"}})
		if got.StartDate == nil || !got.StartDate.Equal(start) {
			t.Fatalf("start_date path did not apply: got %v", got.StartDate)
		}
	})
}
