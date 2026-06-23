package domain

// TagScope distinguishes organizational tags (user-created) from content tags
// (imported from metadata sources).
type TagScope string

// Tag scope constants distinguishing user-created from source-imported tags.
const (
	TagScopeUser     TagScope = "user"
	TagScopeMetadata TagScope = "metadata"
)

// TagKey identifies the semantic namespace of a tag.
type TagKey string

// Tag key constants for well-known namespaces.
const (
	TagKeyGenre             TagKey = "genre"
	TagKeyLabel             TagKey = "label"
	TagKeyNetwork           TagKey = "network"
	TagKeyProductionCompany TagKey = "production_company"
	TagKeyPublisher         TagKey = "publisher"
	TagKeyAdult             TagKey = "adult"
	TagKeyContentWarning    TagKey = "content_warning"
	TagKeyGeneral           TagKey = "tag"
)

// Tag is a label attached to items or library entries for filtering and organization.
type Tag struct {
	ID    string
	Key   TagKey
	Value string
	Scope TagScope
}
