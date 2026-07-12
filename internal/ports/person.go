package ports

import (
	"context"
	"purser/internal/domain"
)

// PersonRepository is the persistence port for domain.Person. It knows
// nothing about any other kernel entity, any transport, or any metadata
// provider — content-type/provider knowledge never belongs here, per
// docs/adr/0001-hexagonal-architecture.md.
//
// List uses opaque cursor pagination: a zero-value pageToken starts from
// the beginning; a non-empty nextPageToken is returned whenever more
// results exist, and is empty on the last page.
type PersonRepository interface {
	Create(ctx context.Context, person *domain.Person) error
	Get(ctx context.Context, id string) (*domain.Person, error)
	Update(ctx context.Context, person *domain.Person) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) (people []*domain.Person, nextPageToken string, err error)
}
