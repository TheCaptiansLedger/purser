package ports

import (
	"context"
	"purser/internal/domain"
)

// MediaFileRepository is the persistence port for domain.MediaFile. See
// PersonRepository for the pagination convention. itemID is an optional
// filter — an empty string means "no filter on this field."
type MediaFileRepository interface {
	Create(ctx context.Context, m *domain.MediaFile) error
	Get(ctx context.Context, id string) (*domain.MediaFile, error)
	Update(ctx context.Context, m *domain.MediaFile) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, itemID string, pageSize int, pageToken string) (mediaFiles []*domain.MediaFile, nextPageToken string, err error)
}
