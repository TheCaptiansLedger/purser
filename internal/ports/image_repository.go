package ports

import (
	"context"
	"purser/internal/domain"
)

// ImageRepository manages persistence for images and user image selections.
type ImageRepository interface {
	// Upsert inserts images, ignoring rows whose URL already exists for the same
	// (entity_type, entity_id, image_type) — safe to call on every re-scan.
	Upsert(ctx context.Context, images []domain.StoredImage) error

	// List returns all images for an entity, optionally filtered by type.
	List(ctx context.Context, entityType, entityID string, imageType *domain.ImageType) ([]domain.StoredImage, error)

	// GetSelection returns the active image for a slot. Returns the explicit user
	// selection when one exists; otherwise falls back to the highest-priority source.
	// Returns nil when no images exist for the slot.
	GetSelection(ctx context.Context, entityType, entityID string, imageType domain.ImageType) (*domain.StoredImage, error)

	// SetSelection records the user's explicit choice for a slot.
	SetSelection(ctx context.Context, entityType, entityID string, imageType domain.ImageType, imageID string) error

	// ClearSelection removes a user selection, reverting to auto-select behavior.
	ClearSelection(ctx context.Context, entityType, entityID string, imageType domain.ImageType) error
}
