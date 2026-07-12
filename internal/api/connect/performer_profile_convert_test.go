package apiconnect

import (
	"purser/internal/domain/afterdark"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

func TestBodyMarksRoundTrip(t *testing.T) {
	if bodyMarksToProto(nil) != nil {
		t.Fatal("bodyMarksToProto(nil) did not return nil")
	}
	if bodyMarksFromProto(nil) != nil {
		t.Fatal("bodyMarksFromProto(nil) did not return nil")
	}

	marks := []afterdark.BodyMark{{Location: "left arm", Description: "rose"}}
	got := bodyMarksFromProto(bodyMarksToProto(marks))
	if len(got) != 1 || got[0].Location != "left arm" || got[0].Description != "rose" {
		t.Fatalf("round trip changed marks: got %+v", got)
	}
}

func TestPerformerProfileRoundTrip(t *testing.T) {
	if performerProfileToProto(nil) != nil {
		t.Fatal("performerProfileToProto(nil) did not return nil")
	}
	if performerProfileFromProto(nil) == nil {
		t.Fatal("performerProfileFromProto(nil) returned nil, want a zero-value PerformerProfile")
	}

	p := &afterdark.PerformerProfile{
		PersonID: "p1", CupSize: "34", BandSize: "C", BreastType: "natural",
		Tattoos:         []afterdark.BodyMark{{Location: "arm", Description: "rose"}},
		Piercings:       []afterdark.BodyMark{{Location: "navel", Description: "ring"}},
		CareerStartYear: 2015, CareerEndYear: 2020,
	}
	got := performerProfileFromProto(performerProfileToProto(p))
	if got.PersonID != p.PersonID || got.CupSize != p.CupSize || got.BandSize != p.BandSize ||
		got.BreastType != p.BreastType || got.CareerStartYear != p.CareerStartYear || got.CareerEndYear != p.CareerEndYear {
		t.Fatalf("round trip changed fields: got %+v", got)
	}
	if len(got.Tattoos) != 1 || got.Tattoos[0].Location != "arm" {
		t.Fatalf("round trip changed Tattoos: got %v", got.Tattoos)
	}
	if len(got.Piercings) != 1 || got.Piercings[0].Location != "navel" {
		t.Fatalf("round trip changed Piercings: got %v", got.Piercings)
	}
}

func TestApplyPerformerProfileFieldMask(t *testing.T) {
	existing := &afterdark.PerformerProfile{PersonID: "p1", CupSize: "Original", BandSize: "Original Band"}

	t.Run("nil mask replaces every mutable field, person_id stays fixed", func(t *testing.T) {
		incoming := &afterdarkv1.PerformerProfile{PersonId: "different", CupSize: "New"}
		got := applyPerformerProfileFieldMask(existing, incoming, nil)
		if got.CupSize != "New" {
			t.Fatalf("nil mask did not replace CupSize: got %+v", got)
		}
		if got.PersonID != "p1" {
			t.Fatalf("mask changed PersonID: got %+v", got)
		}
	})

	t.Run("named path applies only that field", func(t *testing.T) {
		incoming := &afterdarkv1.PerformerProfile{CupSize: "New", BandSize: "Should be ignored"}
		got := applyPerformerProfileFieldMask(existing, incoming, &fieldmaskpb.FieldMask{Paths: []string{"cup_size"}})
		if got.CupSize != "New" || got.BandSize != "Original Band" {
			t.Fatalf("masked update touched the wrong fields: got %+v", got)
		}
	})

	t.Run("every recognized path is applied", func(t *testing.T) {
		full := &afterdarkv1.PerformerProfile{
			CupSize: "c2", BandSize: "b2", BreastType: "t2",
			Tattoos:         bodyMarksToProto([]afterdark.BodyMark{{Location: "l", Description: "d"}}),
			Piercings:       bodyMarksToProto([]afterdark.BodyMark{{Location: "l2", Description: "d2"}}),
			CareerStartYear: 2010, CareerEndYear: 2012,
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{
			"cup_size", "band_size", "breast_type", "tattoos", "piercings",
			"career_start_year", "career_end_year", "unrecognized",
		}}
		got := applyPerformerProfileFieldMask(existing, full, mask)
		if got.CupSize != "c2" || got.BandSize != "b2" || got.BreastType != "t2" ||
			len(got.Tattoos) != 1 || len(got.Piercings) != 1 ||
			got.CareerStartYear != 2010 || got.CareerEndYear != 2012 {
			t.Fatalf("not every recognized path was applied: got %+v", got)
		}
	})
}
