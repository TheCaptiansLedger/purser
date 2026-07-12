package ports

import (
	"context"
	"purser/internal/domain"
)

// ItemPersonRepository is the persistence port for domain.ItemPerson — a
// join row with no independent ID. Its identity is the composite
// (itemID, personID, role), same reasoning as EntryPersonRepository.
//
// List's itemID and personID are independent, optional filters.
type ItemPersonRepository interface {
	Create(ctx context.Context, ip *domain.ItemPerson) error
	Get(ctx context.Context, itemID, personID, role string) (*domain.ItemPerson, error)
	Update(ctx context.Context, ip *domain.ItemPerson) error
	Delete(ctx context.Context, itemID, personID, role string) error
	List(ctx context.Context, itemID, personID string, pageSize int, pageToken string) (itemPeople []*domain.ItemPerson, nextPageToken string, err error)
}
