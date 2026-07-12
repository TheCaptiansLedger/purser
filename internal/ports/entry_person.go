package ports

import (
	"context"
	"purser/internal/domain"
)

// EntryPersonRepository is the persistence port for domain.EntryPerson —
// a join row with no independent ID. Its identity is the composite
// (libraryEntryID, personID, role), so Get/Update/Delete take all three
// instead of a single id string.
//
// List's libraryEntryID and personID are independent, optional filters:
// set one, both, or neither (unfiltered). See ports.PersonRepository for
// the pagination convention.
type EntryPersonRepository interface {
	Create(ctx context.Context, ep *domain.EntryPerson) error
	Get(ctx context.Context, libraryEntryID, personID, role string) (*domain.EntryPerson, error)
	Update(ctx context.Context, ep *domain.EntryPerson) error
	Delete(ctx context.Context, libraryEntryID, personID, role string) error
	List(ctx context.Context, libraryEntryID, personID string, pageSize int, pageToken string) (entryPeople []*domain.EntryPerson, nextPageToken string, err error)
}
