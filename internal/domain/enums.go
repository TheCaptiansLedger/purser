package domain

// ContentType identifies which module a LibraryEntry or Item belongs to.
//
// Deliberately an open string, not a closed set validated against a fixed
// list: per ADR 0001, adding a new content type must be possible with zero
// edits to existing shared code. A oneof-style validator here would make
// every new module a change to this file, which is exactly the violation
// the ADR calls out. The constants below are known values for convenience,
// not an exhaustive list.
type ContentType string

// Known ContentType values, one per module documented so far.
const (
	ContentTypeMovie ContentType = "movie"
	ContentTypeTV    ContentType = "tv"
	ContentTypeMusic ContentType = "music"
	ContentTypeBook  ContentType = "book"
	ContentTypeAdult ContentType = "adult"
)

// Kind identifies what a LibraryEntry represents within its content type
// (e.g. artist, studio, series, author). Open for the same reason as
// ContentType — new content types bring new Kind values without touching
// this file.
type Kind string

// Known Kind values, one per LibraryEntry role documented so far.
const (
	KindNetwork   Kind = "network"
	KindStudio    Kind = "studio"
	KindSeries    Kind = "series"
	KindArtist    Kind = "artist"
	KindAuthor    Kind = "author"
	KindMovie     Kind = "movie"
	KindPublisher Kind = "publisher"
)

// MonitorMode is kernel-owned monitoring semantics, not module-specific
// data, so — unlike ContentType/Kind — it's a closed, validated set.
type MonitorMode string

// The complete set of valid MonitorMode values.
const (
	MonitorModeAll    MonitorMode = "all"
	MonitorModeFuture MonitorMode = "future"
	MonitorModeNone   MonitorMode = "none"
	MonitorModeLatest MonitorMode = "latest"
)

// ItemStatus is acquisition-pipeline state, shared by every content type
// (the pipeline is the core domain, per project convention) — closed,
// validated set.
type ItemStatus string

// The complete set of valid ItemStatus values.
const (
	ItemStatusWanted      ItemStatus = "wanted"
	ItemStatusGrabbed     ItemStatus = "grabbed"
	ItemStatusDownloading ItemStatus = "downloading"
	ItemStatusImported    ItemStatus = "imported"
	ItemStatusMissing     ItemStatus = "missing"
	ItemStatusSkipped     ItemStatus = "skipped"
)

// EntityType names which kernel entity a polymorphic reference (ExternalID,
// CollectionMembership) points at. Closed — this is a fixed, small set of
// kernel-structural attach points, not something new content types extend.
type EntityType string

// The complete set of valid EntityType values.
const (
	EntityTypeLibraryEntry EntityType = "library_entry"
	EntityTypeGroup        EntityType = "group"
	EntityTypeItem         EntityType = "item"
	EntityTypePerson       EntityType = "person"
)

// TagScope distinguishes user-applied organizational tags from
// provider-sourced metadata tags. Closed, kernel-owned.
type TagScope string

// The complete set of valid TagScope values.
const (
	TagScopeUser     TagScope = "user"
	TagScopeMetadata TagScope = "metadata"
)
