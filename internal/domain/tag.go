package domain

// TagScope distinguishes organizational tags (user-created) from content tags
// (imported from metadata sources).
type TagScope string

// Tag scope constants distinguishing user-created from source-imported tags.
const (
	TagScopeUser     TagScope = "user"
	TagScopeMetadata TagScope = "metadata"
)

// Common tag key constants for well-known namespaces.
const (
	TagKeyDefault        = "tag"
	TagKeyGenre          = "genre"
	TagKeyContentWarning = "content_warning"
)

// Tag is a label attached to items or library entries for filtering and organization.
type Tag struct {
	ID    string
	Key   string
	Value string
	Scope TagScope
}
