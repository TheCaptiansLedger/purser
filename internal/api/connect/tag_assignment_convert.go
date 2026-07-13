package apiconnect

import (
	"purser/internal/domain"

	v1 "purser/gen/go/purser/domain/v1"
)

func tagAssignmentToProto(ta *domain.TagAssignment) *v1.TagAssignment {
	if ta == nil {
		return nil
	}
	return &v1.TagAssignment{
		TagId:      ta.TagID,
		EntityType: entityTypeToProto(ta.EntityType),
		EntityId:   ta.EntityID,
	}
}

func tagAssignmentFromProto(pb *v1.TagAssignment) *domain.TagAssignment {
	if pb == nil {
		return &domain.TagAssignment{}
	}
	return &domain.TagAssignment{
		TagID:      pb.GetTagId(),
		EntityType: entityTypeFromProto(pb.GetEntityType()),
		EntityID:   pb.GetEntityId(),
	}
}
