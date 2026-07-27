package domain

import "time"

// UnmatchedFileStatus is kernel-owned pipeline state, closed and validated
// like MonitorMode/ItemStatus in enums.go — scoped to this one entity
// rather than shared across entities, so it's defined here instead, the
// same way Gender is defined in person.go rather than enums.go. See
// docs/adr/0024-pipeline-core.md.
type UnmatchedFileStatus string

// The complete set of valid UnmatchedFileStatus values.
const (
	UnmatchedFileStatusPending   UnmatchedFileStatus = "pending"
	UnmatchedFileStatusMatched   UnmatchedFileStatus = "matched"
	UnmatchedFileStatusDismissed UnmatchedFileStatus = "dismissed"
)

// MatchTier is the pipeline-owned classification of how a MatchCandidate
// was found — direct/unique external IDs are the strongest signal, fuzzy
// and acoustic matches progressively weaker. See
// docs/technical/pipeline-unmatchedfile-grouping.md.
type MatchTier string

// The complete set of valid MatchTier values.
const (
	MatchTierDirectID MatchTier = "direct_id"
	MatchTierUniqueID MatchTier = "unique_id"
	MatchTierFuzzy    MatchTier = "fuzzy"
	MatchTierAcoustic MatchTier = "acoustic"
)

// Fingerprint is the generic, content-type-agnostic identification data a
// FileFingerprinter extracts from a discovered file — raw tags plus
// computed/derived values. Tags/Metadata are open bags, unvalidated
// field-by-field, the same treatment MediaFile.Metadata and Item.Metadata
// already get: they're computed internally by the pipeline, never
// constructed from untrusted client input. See
// docs/adr/0024-pipeline-core.md.
type Fingerprint struct {
	Tags     map[string]string
	Metadata map[string]any
}

// MatchCandidate is one ranked candidate a content-type's identifier
// produced for an UnmatchedFile — the shared decision service compares
// Score against the confidence threshold. Signals/Metadata are open bags,
// same unvalidated treatment as Fingerprint above.
type MatchCandidate struct {
	ExternalRef string
	Title       string
	Score       float64
	Tier        MatchTier
	Signals     map[string]float64
	Metadata    map[string]any
}

// UnmatchedFile is a file the common scan pipeline discovered and hashed
// but has not yet matched to a LibraryEntry/Item — the pre-identification
// state a MediaFile's required ItemID can't represent. See
// docs/adr/0024-pipeline-core.md.
type UnmatchedFile struct {
	ID   string `validate:"required"`
	Path string `validate:"required"`
	Size int64

	OSHash string
	MD5    string
	SHA1   string
	SHA512 string

	// ContentType is the domain.ContentType this file was discovered under
	// (its scan root's configured content type) — needed because a manual
	// AcceptCandidate call happens outside any scan Job's lifetime, so it
	// can't read this off job.Params the way the automatic decide/persist
	// pass does; it must be threaded onto the row itself. Set once by
	// ScanExecutor when the row is first queued. Deliberately not
	// validate:"required": a scan root with no configured
	// config.Pipeline.ScanRoots entry resolves to an empty ContentType
	// (ScanService.resolveContentType's own documented "no match" case),
	// the same "not configured for this content type yet" signal
	// Grouping/FileFingerprinter/Identifier/ConfidenceScore already treat
	// as a legitimate, valid state (falling back to their Identity/Noop
	// implementations) rather than an error — AcceptCandidate on such a
	// group is equally well-defined: PersisterResolver falls back to
	// NoopPersister for an empty/unregistered content type, the same
	// fallback every other pipeline-core capability already gets.
	ContentType ContentType

	// GroupKey identifies the identification unit this file belongs to —
	// files sharing a GroupKey are matched/decided together. Defaults to
	// the file's own Path for ungrouped content types (a group of one, by
	// construction) since grouping runs before ID generation. See
	// docs/technical/pipeline-unmatchedfile-grouping.md.
	GroupKey string `validate:"required"`

	// DiscNumber and TrackNumber are per-row, unlike everything else on
	// this type, which is shared identically across a group. TrackNumber
	// is a string, not an int — matching Item.Sequence's existing
	// convention — to carry vinyl side-lettering ("A1", "B3") the same way
	// MusicBrainz's own track.number does; an int field would silently
	// drop that value on parse failure.
	DiscNumber  int
	TrackNumber string

	Fingerprint *Fingerprint
	Candidates  []MatchCandidate

	DiscoveredAt time.Time
	Status       UnmatchedFileStatus `validate:"required,oneof=pending matched dismissed"`
}

// Validate checks UnmatchedFile's invariants: ID, Path, and GroupKey are
// required, and Status must be one of the known values.
func (u *UnmatchedFile) Validate() error {
	return validateStruct(u)
}
