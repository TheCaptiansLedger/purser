package ports

import (
	"context"
	"purser/internal/domain"
)

// ImageRepository is the persistence port for domain.Image. Unlike the
// join-shaped ports, Image has its own ID (Get/Update/Delete key on it
// directly) — but List still takes ownerType/ownerID as independent,
// optional filters, per docs/adr/0011-api-design.md.
type ImageRepository interface {
	Create(ctx context.Context, img *domain.Image) error
	Get(ctx context.Context, id string) (*domain.Image, error)
	Update(ctx context.Context, img *domain.Image) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, ownerType, ownerID string, pageSize int, pageToken string) (images []*domain.Image, nextPageToken string, err error)
}
