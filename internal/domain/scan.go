package domain

import "time"

// MatchConfidence records how a MediaFile was linked to its Item.
type MatchConfidence string

// Match confidence levels for how a MediaFile was linked to its Item.
const (
	MatchVerified    MatchConfidence = "verified"
	MatchNameMatched MatchConfidence = "name_matched"
	MatchManual      MatchConfidence = "manual"
)

// Fingerprint holds every computed fingerprint for a single scanned file.
// Fields are zero-valued when not computed. A fingerprinter populates only
// the fields it is capable of producing for the file's content type.
type Fingerprint struct {
	OSHash       string
	PHash        string
	AcoustID     string
	EmbeddedTags map[string]string
	ISBN         string
}

// ScannedFile is an intermediate record produced by the scanner before matching.
type ScannedFile struct {
	ID           string
	Path         string
	Size         int64
	ContentType  ContentType
	Fingerprint  *Fingerprint // nil until fingerprinters run
	DiscoveredAt time.Time
}

// MatchCandidate is a potential match for a scanned file. Item is the local
// library record when one exists; ExternalItem carries provider metadata when
// the source identified the file but no local item has been created yet.
// Both may be set (local item found via external lookup); Item may be nil
// when the provider recognised the file but the title is not in the library.
type MatchCandidate struct {
	Item         *Item
	ExternalItem *ExternalItem
	Confidence   float64 // 0.0–1.0
	Source       string  // strategy: "oshash", "acoustid", "isbn_provider", "filename", etc.
}

// UnmatchedFile is written to the queue when no candidate meets the auto-import threshold.
type UnmatchedFile struct {
	ID           string
	Path         string
	Size         int64
	ContentType  ContentType
	Fingerprint  *Fingerprint
	Candidates   []MatchCandidate // ranked by Confidence desc; may be empty
	DiscoveredAt time.Time
	Status       UnmatchedStatus
	// DuplicateOf holds the MediaFile.ID of an already-imported file with the same
	// content hash or the same target item, indicating this entry is a duplicate or
	// upgrade candidate rather than a wholly new file.
	DuplicateOf   string
	ThumbnailPath string // local path to a cached thumbnail for queue display
}

// UnmatchedStatus tracks the resolution state of an unmatched file queue entry.
type UnmatchedStatus string

// Unmatched file resolution states.
const (
	UnmatchedPending   UnmatchedStatus = "pending"
	UnmatchedMatched   UnmatchedStatus = "matched"
	UnmatchedDismissed UnmatchedStatus = "dismissed"
)

// UnmatchedFileGroup aggregates unmatched music files that share the same
// candidate album (GroupID). Used by the groupBy=album list endpoint.
type UnmatchedFileGroup struct {
	GroupID                 string
	GroupTitle              string
	Files                   []*UnmatchedFile
	BestCandidateConfidence float64
}
