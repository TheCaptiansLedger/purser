package apiconnect

import (
	v1 "purser/gen/go/purser/domain/v1"
	"purser/internal/domain"
)

func entityTypeToProto(e domain.EntityType) v1.EntityType {
	switch e {
	case domain.EntityTypeLibraryEntry:
		return v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY
	case domain.EntityTypeGroup:
		return v1.EntityType_ENTITY_TYPE_GROUP
	case domain.EntityTypeItem:
		return v1.EntityType_ENTITY_TYPE_ITEM
	case domain.EntityTypePerson:
		return v1.EntityType_ENTITY_TYPE_PERSON
	default:
		return v1.EntityType_ENTITY_TYPE_UNSPECIFIED
	}
}

func entityTypeFromProto(e v1.EntityType) domain.EntityType {
	switch e {
	case v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY:
		return domain.EntityTypeLibraryEntry
	case v1.EntityType_ENTITY_TYPE_GROUP:
		return domain.EntityTypeGroup
	case v1.EntityType_ENTITY_TYPE_ITEM:
		return domain.EntityTypeItem
	case v1.EntityType_ENTITY_TYPE_PERSON:
		return domain.EntityTypePerson
	default:
		return domain.EntityType("")
	}
}

func externalIDToProto(e *domain.ExternalID) *v1.ExternalID {
	if e == nil {
		return nil
	}
	return &v1.ExternalID{
		EntityType: entityTypeToProto(e.EntityType),
		EntityId:   e.EntityID,
		Source:     string(e.Source),
		Value:      e.Value,
	}
}

func externalIDFromProto(pb *v1.ExternalID) *domain.ExternalID {
	if pb == nil {
		return &domain.ExternalID{}
	}
	return &domain.ExternalID{
		EntityType: entityTypeFromProto(pb.GetEntityType()),
		EntityID:   pb.GetEntityId(),
		Source:     domain.ExternalIDSource(pb.GetSource()),
		Value:      pb.GetValue(),
	}
}
