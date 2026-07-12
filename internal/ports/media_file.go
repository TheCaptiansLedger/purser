package ports

import (
	"context"
	"purser/internal/domain"
)

// MediaFileRepository is the persistence port for domain.MediaFile. See
// PersonRepository for the pagination convention.
type MediaFileRepository interface {
	Create(ctx context.Context, m *domain.MediaFile) error
	Get(ctx context.Context, id string) (*domain.MediaFile, error)
	Update(ctx context.Context, m *domain.MediaFile) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) (mediaFiles []*domain.MediaFile, nextPageToken string, err error)
}
