package ports

import (
	"context"
	"purser/internal/domain"
)

// GroupRepository is the persistence port for domain.Group. See
// PersonRepository for the pagination convention.
type GroupRepository interface {
	Create(ctx context.Context, g *domain.Group) error
	Get(ctx context.Context, id string) (*domain.Group, error)
	Update(ctx context.Context, g *domain.Group) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) (groups []*domain.Group, nextPageToken string, err error)
}
