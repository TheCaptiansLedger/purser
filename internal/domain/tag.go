package domain

// TagScope distinguishes organizational tags (user-created) from content tags
// (imported from metadata sources).
type TagScope string

// Tag scope constants distinguishing user-created from source-imported tags.
const (
	TagScopeUser     TagScope = "user"
	TagScopeMetadata TagScope = "metadata"
)

// TagCategory classifies what kind of metadata a tag represents.
// The empty string means general / uncategorized.
type TagCategory string

// Tag category constants for classifying what kind of metadata a tag represents.
const (
	TagCategoryGenre          TagCategory = "genre"
	TagCategoryContentWarning TagCategory = "content_warning"
)

// Tag is a label attached to items or library entries for filtering and organization.
type Tag struct {
	ID       string
	Name     string
	Scope    TagScope
	Category TagCategory
}
