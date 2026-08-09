package domain

// ExternalIDSource names which external provider an ExternalID points
// into — MusicBrainz, StashDB, TMDB, whatever. Deliberately an open
// string, not a closed set: per ADR 0001, a new metadata provider must be
// addable with zero edits to shared code, and a oneof-style validator here
// would violate that the first time a new provider showed up. The
// constants below are known values found across the five module data
// model docs, not an exhaustive list.
type ExternalIDSource string

// Known ExternalIDSource values, one per metadata provider covered by the
// five module data model docs.
const (
	ExternalIDSourceMBZ          ExternalIDSource = "mbz"
	ExternalIDSourceMBZRecording ExternalIDSource = "mbz_recording"
	ExternalIDSourceAudioDB      ExternalIDSource = "audiodb"
	ExternalIDSourceStashDB      ExternalIDSource = "stashdb"
	ExternalIDSourceTPDB         ExternalIDSource = "tpdb"
	// ExternalIDSourceJAVCode is a content-derived (not provider-issued)
	// JAV product code — e.g. "SSIS-001" — extracted from a filename by
	// internal/adapters/pipeline/afterdark.ExtractJAVCode. Unlike every
	// other ExternalIDSource, its Value never comes from a provider
	// response; it exists so the AfterDark Persister's get-or-create can
	// recognize a previously-imported scene across a provider-ID change
	// (a rescan that clears threshold via a different provider than the
	// one that originally created the Item), see AD7 (issue #561).
	ExternalIDSourceJAVCode     ExternalIDSource = "jav_code"
	ExternalIDSourceTMDB        ExternalIDSource = "tmdb"
	ExternalIDSourceTVDB        ExternalIDSource = "tvdb"
	ExternalIDSourceOMDB        ExternalIDSource = "omdb"
	ExternalIDSourceOpenLibrary ExternalIDSource = "openlibrary"
	ExternalIDSourceHardcover   ExternalIDSource = "hardcover"
)

// ExternalID is a polymorphic reference: EntityType + EntityID identify
// which kernel record this points at, Source + Value identify where in the
// outside world it points to.
type ExternalID struct {
	EntityType EntityType       `validate:"required,oneof=library_entry group item person"`
	EntityID   string           `validate:"required"`
	Source     ExternalIDSource `validate:"required"`
	Value      string           `validate:"required"`
}

// Validate checks ExternalID's invariants: EntityType must be one of the
// kernel's known attach points, and EntityID/Source/Value are all required.
func (e *ExternalID) Validate() error {
	return validateStruct(e)
}
