package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "purser/gen/go/purser/domain/v1"
)

func entryPersonToProto(ep *domain.EntryPerson) *v1.EntryPerson {
	if ep == nil {
		return nil
	}
	pb := &v1.EntryPerson{
		LibraryEntryId: ep.LibraryEntryID,
		PersonId:       ep.PersonID,
		Role:           ep.Role,
		CreditedAs:     ep.CreditedAs,
		Character:      ep.Character,
	}
	if ep.StartDate != nil {
		pb.StartDate = timestamppb.New(*ep.StartDate)
	}
	if ep.EndDate != nil {
		pb.EndDate = timestamppb.New(*ep.EndDate)
	}
	return pb
}

func entryPersonFromProto(pb *v1.EntryPerson) *domain.EntryPerson {
	if pb == nil {
		return &domain.EntryPerson{}
	}
	ep := &domain.EntryPerson{
		LibraryEntryID: pb.GetLibraryEntryId(),
		PersonID:       pb.GetPersonId(),
		Role:           pb.GetRole(),
		CreditedAs:     pb.GetCreditedAs(),
		Character:      pb.GetCharacter(),
	}
	if pb.GetStartDate() != nil {
		t := pb.GetStartDate().AsTime()
		ep.StartDate = &t
	}
	if pb.GetEndDate() != nil {
		t := pb.GetEndDate().AsTime()
		ep.EndDate = &t
	}
	return ep
}

// applyEntryPersonFieldMask merges incoming onto a copy of existing,
// restricted to the field-mask paths named. library_entry_id, person_id,
// and role identify the row and are deliberately not switch cases here —
// see entry_person.proto's UpdateEntryPersonRequest comment.
func applyEntryPersonFieldMask(existing *domain.EntryPerson, incoming *v1.EntryPerson, mask *fieldmaskpb.FieldMask) *domain.EntryPerson {
	full := entryPersonFromProto(incoming)
	full.LibraryEntryID = existing.LibraryEntryID
	full.PersonID = existing.PersonID
	full.Role = existing.Role

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		switch path {
		case "credited_as":
			merged.CreditedAs = full.CreditedAs
		case "character":
			merged.Character = full.Character
		case "start_date":
			merged.StartDate = full.StartDate
		case "end_date":
			merged.EndDate = full.EndDate
		}
	}
	return &merged
}
