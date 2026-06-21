package ports

import (
	"context"
	"purser/internal/domain"
)

// LibraryFilter specifies criteria for listing LibraryEntries.
type LibraryFilter struct {
	ContentType domain.ContentType
	Kind        domain.Kind
	ParentID    string
	PersonID    string // entries where this person is a member (via entry_people)
	Monitored   *bool
	Search      string
	Limit       int
	Offset      int
}

// LibraryEntryRepository manages persistence for the content hierarchy.
type LibraryEntryRepository interface {
	Get(ctx context.Context, id string) (*domain.LibraryEntry, error)
	List(ctx context.Context, f LibraryFilter) ([]*domain.LibraryEntry, int, error)
	Save(ctx context.Context, e *domain.LibraryEntry) error
	Delete(ctx context.Context, id string) error
	GetPeople(ctx context.Context, entryID string) ([]domain.EntryPerson, error)
	SavePerson(ctx context.Context, entryID string, ep domain.EntryPerson) error
	RemovePerson(ctx context.Context, entryID, personID, role string) error
}

// GroupFilter specifies criteria for listing Groups.
type GroupFilter struct {
	LibraryEntryID string
	Monitored      *bool
}

// GroupRepository manages persistence for groups (seasons, albums, series).
type GroupRepository interface {
	Get(ctx context.Context, id string) (*domain.Group, error)
	List(ctx context.Context, f GroupFilter) ([]*domain.Group, error)
	Save(ctx context.Context, g *domain.Group) error
	Delete(ctx context.Context, id string) error
}

// ItemFilter specifies criteria for listing Items.
type ItemFilter struct {
	LibraryEntryID string
	GroupID        string
	ContentType    domain.ContentType
	Status         domain.ItemStatus
	Monitored      *bool
	PersonID       string   // items featuring this person
	TagIDs         []string // items carrying all of these tags
	Search         string
	Sort           string // "date" | "title"; adapter defaults to "date"
	SortDir        string // "asc" | "desc"; adapter defaults to "desc"
	Limit          int
	Offset         int
}

// ItemRepository manages persistence for leaf items (episodes, scenes, tracks, etc.).
type ItemRepository interface {
	Get(ctx context.Context, id string) (*domain.Item, error)
	List(ctx context.Context, f ItemFilter) ([]*domain.Item, int, error)
	Save(ctx context.Context, item *domain.Item) error
	Delete(ctx context.Context, id string) error
}

// PersonFilter specifies criteria for listing People.
type PersonFilter struct {
	ContentType domain.ContentType
	Monitored   *bool
	Role        domain.PersonRole
	Search      string
	Limit       int
	Offset      int
}

// PersonRepository manages persistence for people (performers, cast, artists, actresses).
type PersonRepository interface {
	Get(ctx context.Context, id string) (*domain.Person, error)
	List(ctx context.Context, f PersonFilter) ([]*domain.Person, int, error)
	Save(ctx context.Context, p *domain.Person) error
	Delete(ctx context.Context, id string) error
}

// TagFilter specifies criteria for listing tags.
type TagFilter struct {
	Scope        domain.TagScope
	Category     domain.TagCategory
	ContentTypes []domain.ContentType // when set, only tags used by one of these content types are returned
}

// TagRepository manages persistence for tags.
type TagRepository interface {
	Get(ctx context.Context, id string) (*domain.Tag, error)
	List(ctx context.Context, f TagFilter) ([]*domain.Tag, error)
	Save(ctx context.Context, t *domain.Tag) error
	Delete(ctx context.Context, id string) error
}

// ExternalIDRepository resolves external source identifiers to internal entity IDs.
// Used by import flows to detect whether a remote entity is already in the library.
type ExternalIDRepository interface {
	// FindEntity returns the internal entity ID for (entityType, source, value).
	// Returns errs.ErrNotFound when no match exists.
	FindEntity(ctx context.Context, entityType, source, value string) (string, error)
}

// MediaFileRepository manages persistence for on-disk file records.
type MediaFileRepository interface {
	GetByItemID(ctx context.Context, itemID string) (*domain.MediaFile, error)
	GetByOSHash(ctx context.Context, hash string) (*domain.MediaFile, error)
	Save(ctx context.Context, f *domain.MediaFile) error
	Delete(ctx context.Context, id string) error
}

// SettingsRepository persists runtime configuration key-value pairs.
type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, error) // returns errs.ErrNotFound if missing
	Set(ctx context.Context, key, value string) error
}
