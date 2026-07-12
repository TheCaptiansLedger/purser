package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func tagScopeToProto(s domain.TagScope) v1.TagScope {
	switch s {
	case domain.TagScopeUser:
		return v1.TagScope_TAG_SCOPE_USER
	case domain.TagScopeMetadata:
		return v1.TagScope_TAG_SCOPE_METADATA
	default:
		return v1.TagScope_TAG_SCOPE_UNSPECIFIED
	}
}

func tagScopeFromProto(s v1.TagScope) domain.TagScope {
	switch s {
	case v1.TagScope_TAG_SCOPE_USER:
		return domain.TagScopeUser
	case v1.TagScope_TAG_SCOPE_METADATA:
		return domain.TagScopeMetadata
	default:
		return domain.TagScope("")
	}
}

func tagToProto(t *domain.Tag) *v1.Tag {
	if t == nil {
		return nil
	}
	return &v1.Tag{
		Id:       t.ID,
		Key:      t.Key,
		Value:    t.Value,
		Scope:    tagScopeToProto(t.Scope),
		Category: t.Category,
	}
}

func tagFromProto(pb *v1.Tag) *domain.Tag {
	if pb == nil {
		return &domain.Tag{}
	}
	return &domain.Tag{
		ID:       pb.GetId(),
		Key:      pb.GetKey(),
		Value:    pb.GetValue(),
		Scope:    tagScopeFromProto(pb.GetScope()),
		Category: pb.GetCategory(),
	}
}

// applyTagFieldMask merges incoming onto a copy of existing, restricted
// to the field-mask paths named. See applyPersonFieldMask for the
// convention.
func applyTagFieldMask(existing *domain.Tag, incoming *v1.Tag, mask *fieldmaskpb.FieldMask) *domain.Tag {
	full := tagFromProto(incoming)
	full.ID = existing.ID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		switch path {
		case "key":
			merged.Key = full.Key
		case "value":
			merged.Value = full.Value
		case "scope":
			merged.Scope = full.Scope
		case "category":
			merged.Category = full.Category
		}
	}
	return &merged
}
