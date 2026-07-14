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
}
