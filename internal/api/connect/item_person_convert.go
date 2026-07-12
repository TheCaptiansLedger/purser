package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func itemPersonToProto(ip *domain.ItemPerson) *v1.ItemPerson {
	if ip == nil {
		return nil
	}
	return &v1.ItemPerson{
		ItemId:     ip.ItemID,
		PersonId:   ip.PersonID,
		Role:       ip.Role,
		CreditedAs: ip.CreditedAs,
		Character:  ip.Character,
	}
}

func itemPersonFromProto(pb *v1.ItemPerson) *domain.ItemPerson {
	if pb == nil {
		return &domain.ItemPerson{}
	}
	return &domain.ItemPerson{
		ItemID:     pb.GetItemId(),
		PersonID:   pb.GetPersonId(),
		Role:       pb.GetRole(),
		CreditedAs: pb.GetCreditedAs(),
		Character:  pb.GetCharacter(),
	}
}

// applyItemPersonFieldMask merges incoming onto a copy of existing,
// restricted to the field-mask paths named. item_id, person_id, and role
// identify the row and are deliberately not switch cases here.
func applyItemPersonFieldMask(existing *domain.ItemPerson, incoming *v1.ItemPerson, mask *fieldmaskpb.FieldMask) *domain.ItemPerson {
	full := itemPersonFromProto(incoming)
	full.ItemID = existing.ItemID
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
		}
	}
	return &merged
}
