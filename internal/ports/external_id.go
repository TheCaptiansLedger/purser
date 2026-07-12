package ports

import (
	"context"
	"purser/internal/domain"
)

// ExternalIDRepository is the persistence port for domain.ExternalID — a
// join row with no independent ID. Its identity is the composite
// (entityType, entityID, source); only Value is mutable via Update.
//
// List's entityType and entityID are independent, optional filters.
type ExternalIDRepository interface {
	Create(ctx context.Context, e *domain.ExternalID) error
	Get(ctx context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error)
	Update(ctx context.Context, e *domain.ExternalID) error
	Delete(ctx context.Context, entityType domain.EntityType, entityID, source string) error
	List(ctx context.Context, entityType domain.EntityType, entityID string, pageSize int, pageToken string) (externalIDs []*domain.ExternalID, nextPageToken string, err error)
}
