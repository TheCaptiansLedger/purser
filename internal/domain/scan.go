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

// MatchCandidate is a library item that a FileIdentifier believes may correspond
// to a ScannedFile, with a confidence score.
type MatchCandidate struct {
	Item       *Item
	Confidence float64 // 0.0–1.0
	Source     string  // strategy that produced this: "oshash", "acoustid", "tags", "filename", etc.
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
