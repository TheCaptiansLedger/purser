package domain

import "time"

// Item is the leaf of the content hierarchy — a Track, a Scene, an
// Episode, a Movie's single auto-created item, a book Edition. ContentType
// is what a module-specific query filters on; GroupID is nullable (a movie
// has none, a track always has one).
//
// Status is acquisition-pipeline state, shared by every content type
// regardless of what the content actually is — the pipeline is the core
// domain, not a module concern.
type Item struct {
	ID             string      `validate:"required"`
	ContentType    ContentType `validate:"required"`
	LibraryEntryID string      `validate:"required"`
	GroupID        string
	Title          string `validate:"required"`
	Overview       string
	Date           *time.Time
	Sequence       string
	RuntimeSeconds int

	Monitored bool
	Status    ItemStatus `validate:"required,oneof=wanted grabbed downloading imported missing skipped"`

	Metadata map[string]any
}

// Validate checks Item's invariants: ID, ContentType, LibraryEntryID, and
// Title are required, and Status must be one of the known pipeline states.
func (i *Item) Validate() error {
	return validateStruct(i)
}
