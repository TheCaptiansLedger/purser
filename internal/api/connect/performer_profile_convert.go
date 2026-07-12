package apiconnect

import (
	"purser/internal/domain/afterdark"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

func bodyMarksToProto(marks []afterdark.BodyMark) []*afterdarkv1.BodyMark {
	if len(marks) == 0 {
		return nil
	}
	pb := make([]*afterdarkv1.BodyMark, 0, len(marks))
	for _, m := range marks {
		pb = append(pb, &afterdarkv1.BodyMark{Location: m.Location, Description: m.Description})
	}
	return pb
}

func bodyMarksFromProto(pb []*afterdarkv1.BodyMark) []afterdark.BodyMark {
	if len(pb) == 0 {
		return nil
	}
	marks := make([]afterdark.BodyMark, 0, len(pb))
	for _, m := range pb {
		marks = append(marks, afterdark.BodyMark{Location: m.GetLocation(), Description: m.GetDescription()})
	}
	return marks
}

func performerProfileToProto(p *afterdark.PerformerProfile) *afterdarkv1.PerformerProfile {
	if p == nil {
		return nil
	}
	return &afterdarkv1.PerformerProfile{
		PersonId:        p.PersonID,
		CupSize:         p.CupSize,
		BandSize:        p.BandSize,
		BreastType:      p.BreastType,
		Tattoos:         bodyMarksToProto(p.Tattoos),
		Piercings:       bodyMarksToProto(p.Piercings),
		CareerStartYear: toInt32(p.CareerStartYear),
		CareerEndYear:   toInt32(p.CareerEndYear),
	}
}

func performerProfileFromProto(pb *afterdarkv1.PerformerProfile) *afterdark.PerformerProfile {
	if pb == nil {
		return &afterdark.PerformerProfile{}
	}
	return &afterdark.PerformerProfile{
		PersonID:        pb.GetPersonId(),
		CupSize:         pb.GetCupSize(),
		BandSize:        pb.GetBandSize(),
		BreastType:      pb.GetBreastType(),
		Tattoos:         bodyMarksFromProto(pb.GetTattoos()),
		Piercings:       bodyMarksFromProto(pb.GetPiercings()),
		CareerStartYear: int(pb.GetCareerStartYear()),
		CareerEndYear:   int(pb.GetCareerEndYear()),
	}
}

// applyPerformerProfileFieldMask merges incoming onto a copy of existing,
// restricted to the field-mask paths named. person_id identifies the row
// and is deliberately not a switch case here. See applyPersonFieldMask
// for the convention.
func applyPerformerProfileFieldMask(existing *afterdark.PerformerProfile, incoming *afterdarkv1.PerformerProfile, mask *fieldmaskpb.FieldMask) *afterdark.PerformerProfile {
	full := performerProfileFromProto(incoming)
	full.PersonID = existing.PersonID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	setters := map[string]func(){
		"cup_size":          func() { merged.CupSize = full.CupSize },
		"band_size":         func() { merged.BandSize = full.BandSize },
		"breast_type":       func() { merged.BreastType = full.BreastType },
		"tattoos":           func() { merged.Tattoos = full.Tattoos },
		"piercings":         func() { merged.Piercings = full.Piercings },
		"career_start_year": func() { merged.CareerStartYear = full.CareerStartYear },
		"career_end_year":   func() { merged.CareerEndYear = full.CareerEndYear },
	}
	for _, path := range mask.GetPaths() {
		if set, ok := setters[path]; ok {
			set()
		}
	}
	return &merged
}
