package ports

import (
	"context"
	"purser/internal/domain"
)

// TagAssignmentRepository is the persistence port for domain.TagAssignment
// — a join row with no independent ID. Its identity is the composite
// (tagID, entityType, entityID). There is no Update: nothing about a
// TagAssignment is mutable, unlike EntryPerson/ItemPerson's CreditedAs —
// it either exists or it doesn't.
//
// List's tagID and (entityType, entityID) are independent, optional filter
// directions — "everything tagged X" (browse-by-tag) vs. "this entity's
// tags" (show-an-entity's-tags) — both need to be real indexed lookups,
// per docs/technical/tag-assignment.md Section 3 and
// docs/adr/0012-datastore-persistence.md's CompositeRepository[T] indexing
// addendum.
type TagAssignmentRepository interface {
	Create(ctx context.Context, ta *domain.TagAssignment) error
	Get(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) (*domain.TagAssignment, error)
	Delete(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) error
	List(ctx context.Context, tagID string, entityType domain.EntityType, entityID string, pageSize int, pageToken string) (assignments []*domain.TagAssignment, nextPageToken string, err error)
}
