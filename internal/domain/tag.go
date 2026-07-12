package domain

// Tag is a global, cross-module label. The same (Key, Value) row is what
// makes "show me every movie, book, and scene tagged genre:gonzo" one join
// shape instead of five — content type lives on the item being tagged, not
// on the tag itself.
//
// Category is optional: StashDB's tags carry a category/group (e.g.
// "Finishers" under "ACTION") that flat Key/Value can't express on their
// own. Left empty, everything that only ever used Key/Value is unaffected.
type Tag struct {
	ID       string   `validate:"required"`
	Key      string   `validate:"required"`
	Value    string   `validate:"required"`
	Scope    TagScope `validate:"required,oneof=user metadata"`
	Category string
}

// Validate checks Tag's invariants: ID, Key, and Value are required, and
// Scope must be one of the known scopes.
func (t *Tag) Validate() error {
	return validateStruct(t)
}
