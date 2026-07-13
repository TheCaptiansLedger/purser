package domain

// TagAssignment is a polymorphic Tag attachment — a join row with no
// independent ID, same convention as EntryPerson/ItemPerson/ExternalID.
// TagID identifies the tag; EntityType+EntityID identify what it's
// attached to, reusing the same closed EntityType enum ExternalID already
// uses (library_entry, group, item, person) rather than a Tag-specific
// vocabulary — see docs/technical/tag-assignment.md. This is what lets a
// Network, a Studio, a Scene, and a Performer all be tagged through one
// mechanism instead of one join table per attach point.
type TagAssignment struct {
	TagID      string     `validate:"required"`
	EntityType EntityType `validate:"required,oneof=library_entry group item person"`
	EntityID   string     `validate:"required"`
}

// Validate checks TagAssignment's invariants: TagID and EntityID are
// required, and EntityType must be one of the kernel's known attach points.
func (t *TagAssignment) Validate() error {
	return validateStruct(t)
}
