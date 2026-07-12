package apiconnect

import (
	"purser/internal/domain"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func TestItemPersonRoundTrip(t *testing.T) {
	if itemPersonToProto(nil) != nil {
		t.Fatal("itemPersonToProto(nil) did not return nil")
	}
	if itemPersonFromProto(nil) == nil {
		t.Fatal("itemPersonFromProto(nil) returned nil, want a zero-value ItemPerson")
	}

	ip := &domain.ItemPerson{ItemID: "i1", PersonID: "p1", Role: "performer", CreditedAs: "Credit", Character: "Char"}
	got := itemPersonFromProto(itemPersonToProto(ip))
	if got.ItemID != ip.ItemID || got.PersonID != ip.PersonID || got.Role != ip.Role ||
		got.CreditedAs != ip.CreditedAs || got.Character != ip.Character {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
}

func TestApplyItemPersonFieldMask(t *testing.T) {
	existing := &domain.ItemPerson{ItemID: "i1", PersonID: "p1", Role: "performer", CreditedAs: "Original", Character: "Original Char"}

	t.Run("nil mask replaces every mutable field, key fields stay fixed", func(t *testing.T) {
		incoming := &v1.ItemPerson{ItemId: "different", PersonId: "different", Role: "different", CreditedAs: "New"}
		got := applyItemPersonFieldMask(existing, incoming, nil)
		if got.CreditedAs != "New" {
			t.Fatalf("nil mask did not replace CreditedAs: got %+v", got)
		}
		if got.ItemID != "i1" || got.PersonID != "p1" || got.Role != "performer" {
			t.Fatalf("mask changed the composite key fields: got %+v", got)
		}
	})

	t.Run("named path applies only that field", func(t *testing.T) {
		incoming := &v1.ItemPerson{CreditedAs: "New", Character: "Should be ignored"}
		got := applyItemPersonFieldMask(existing, incoming, &fieldmaskpb.FieldMask{Paths: []string{"credited_as"}})
		if got.CreditedAs != "New" || got.Character != "Original Char" {
			t.Fatalf("masked update touched the wrong fields: got %+v", got)
		}
	})
}
