package ports

import (
	"context"
	"purser/internal/domain"
)

// ExternalIDRepository is the persistence port for domain.ExternalID — a
// join row with no independent ID. Its storage identity is the composite
// (entityType, entityID, source); only Value is mutable via Update.
//
// Create is get-or-create against the separate identity (entityType,
// source, value): if a row already links that source/value pair to some
// entity, e is mutated in place to the existing row's fields (including its
// EntityID) and Create returns nil — the caller inspects e.EntityID
// afterward to learn who actually won. A genuinely new (entityType,
// source, value) is created as given; Create still returns ports.ErrConflict
// if e's own (entityType, entityID, source) storage key is already taken by
// a different Value (use Update instead). See
// docs/adr/0026-external-id-get-or-create.md.
//
// List's entityType and entityID are independent, optional filters.
type ExternalIDRepository interface {
	Create(ctx context.Context, e *domain.ExternalID) error
	Get(ctx context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error)
	// GetByValue returns the ExternalID row currently linking source/value
	// to an entity of entityType, or ports.ErrNotFound if none does.
	GetByValue(ctx context.Context, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, error)
	Update(ctx context.Context, e *domain.ExternalID) error
	Delete(ctx context.Context, entityType domain.EntityType, entityID, source string) error
	List(ctx context.Context, entityType domain.EntityType, entityID string, pageSize int, pageToken string) (externalIDs []*domain.ExternalID, nextPageToken string, err error)
}
