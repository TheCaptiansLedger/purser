package ports

import (
	"context"
	"purser/internal/domain"
)

// ImageSelectionRepository is the persistence port for
// domain.ImageSelection — one row per (ownerType, ownerID, imageType)
// slot, looked up and removed by that triple directly rather than by an
// opaque ID, since ImageSelection has none of its own (see the type's own
// doc comment). No List: nothing ever needs "every selection," only "the
// one for this slot."
type ImageSelectionRepository interface {
	Create(ctx context.Context, sel *domain.ImageSelection) error
	Get(ctx context.Context, ownerType, ownerID string, imageType domain.ImageType) (*domain.ImageSelection, error)
	Update(ctx context.Context, sel *domain.ImageSelection) error
	Delete(ctx context.Context, ownerType, ownerID string, imageType domain.ImageType) error
}
