package domain

// Tag is a global, cross-module label. The same (Scope, Key, Value) row is
// what makes "show me every movie, book, and scene tagged genre:gonzo" one
// join shape instead of five — content type lives on the item being tagged,
// not on the tag itself. (Scope, Key, Value) is enforced as a uniqueness
// constraint at the adapter layer (see
// docs/adr/0019-tag-identity-and-get-or-create.md) — creating a Tag that
// already exists returns the existing row rather than a duplicate.
//
// Category is optional and explicitly not part of identity: StashDB's tags
// carry a category/group (e.g. "Finishers" under "ACTION") that flat
// Key/Value can't express on their own, but it's a cosmetic passenger
// field, never queried or grouped on. Left empty, everything that only
// ever used Key/Value is unaffected.
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
