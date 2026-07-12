package domain

// Group is the tier between a LibraryEntry and its Items — an Album under
// an Artist, a Season under a Series, a Work under an Author. Belongs to
// exactly one LibraryEntry; cardinality (zero, one, or many per entry) is
// what varies per module, not the mechanism itself.
//
// Number is a string, not an int, matching the treatment Item.Sequence
// already has for vinyl side-lettering ("A1") — needed for real,
// non-integer ordering (Hardcover's fractional book-series positions).
// Module-specific alternate numbering (TVDB's aired/DVD/absolute order)
// belongs in Metadata, never a new field here.
type Group struct {
	ID             string `validate:"required"`
	LibraryEntryID string `validate:"required"`
	Title          string `validate:"required"`
	SortName       string
	Number         string
	Year           int
	Overview       string

	Monitored   bool
	MonitorMode MonitorMode `validate:"required,oneof=all future none latest"`

	Metadata map[string]any
}

// Validate checks Group's invariants: ID, LibraryEntryID, and Title are
// required, and MonitorMode must be one of the known monitoring modes.
func (g *Group) Validate() error {
	return validateStruct(g)
}
