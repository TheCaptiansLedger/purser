package ports

import (
	"context"
	"purser/internal/domain"
)

// GroupRepository is the persistence port for domain.Group. See
// PersonRepository for the pagination convention. libraryEntryID is an
// optional filter — an empty string means "no filter on this field."
type GroupRepository interface {
	Create(ctx context.Context, g *domain.Group) error
	Get(ctx context.Context, id string) (*domain.Group, error)
	Update(ctx context.Context, g *domain.Group) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, libraryEntryID string, pageSize int, pageToken string) (groups []*domain.Group, nextPageToken string, err error)
}
