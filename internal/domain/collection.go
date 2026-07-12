package domain

// Collection is an ordered, named set that cuts across the normal
// LibraryEntry -> Group -> Item tree instead of nesting inside it — a
// movie franchise spanning several independent LibraryEntry records, or a
// book series spanning several Work-level Group records. See
// docs/technical/shared-domain-model.md Part 1 for why this isn't just an
// optional Group.
//
// Kind is an open string ("franchise", "book_series", ...), not a closed
// enum — only two modules use it so far, not enough to justify locking the
// vocabulary down.
type Collection struct {
	ID       string `validate:"required"`
	Name     string `validate:"required"`
	Kind     string `validate:"required"`
	Overview string
}

// Validate checks Collection's invariants: ID, Name, and Kind are required.
func (c *Collection) Validate() error {
	return validateStruct(c)
}

// CollectionMembership attaches a LibraryEntry or Group to a Collection,
// with a Position supporting fractional/lettered ordering (Hardcover's
// "1.5" for a novella between two numbered books).
//
// EntityType is deliberately limited to library_entry and group — a
// Collection attaches at whichever level a module's unit-of-participation
// actually lives (movies at LibraryEntry, books at Group), never at Item.
type CollectionMembership struct {
	CollectionID string     `validate:"required"`
	EntityType   EntityType `validate:"required,oneof=library_entry group"`
	EntityID     string     `validate:"required"`
	Position     string
}

// Validate checks CollectionMembership's invariants: CollectionID and
// EntityID are required, and EntityType must be library_entry or group.
func (m *CollectionMembership) Validate() error {
	return validateStruct(m)
}
