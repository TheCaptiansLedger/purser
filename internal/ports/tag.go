package ports

import (
	"context"
	"purser/internal/domain"
)

// TagRepository is the persistence port for domain.Tag's catalog CRUD.
// Polymorphic attach/detach to an entity is TagAssignmentRepository — see
// docs/technical/tag-assignment.md.
type TagRepository interface {
	Create(ctx context.Context, t *domain.Tag) error
	Get(ctx context.Context, id string) (*domain.Tag, error)
	Update(ctx context.Context, t *domain.Tag) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) (tags []*domain.Tag, nextPageToken string, err error)

	// DeleteBatch removes every Tag whose ID is in ids, atomically — all
	// succeed or none do. Tag is one of the two entities ADR 0016 names for
	// a real bulk-delete UI use case; most entities never need this.
	DeleteBatch(ctx context.Context, ids []string) error
}
