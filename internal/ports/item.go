package ports

import (
	"context"
	"purser/internal/domain"
)

// ItemRepository is the persistence port for domain.Item. See
// PersonRepository for the pagination convention. libraryEntryID,
// contentType, and groupID are independent, optional filters — an empty
// string means "no filter on this field."
type ItemRepository interface {
	Create(ctx context.Context, i *domain.Item) error
	Get(ctx context.Context, id string) (*domain.Item, error)
	Update(ctx context.Context, i *domain.Item) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, libraryEntryID, contentType, groupID string, pageSize int, pageToken string) (items []*domain.Item, nextPageToken string, err error)
}
