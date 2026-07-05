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

// MatchSource identifies the strategy used to produce a MatchCandidate.
type MatchSource string

// Known match source constants. Adapters set MatchCandidate.Source to one of these.
const (
	MatchSourceOSHash       MatchSource = "oshash"
	MatchSourceProviderHash MatchSource = "provider_hash"
	MatchSourceAcoustID     MatchSource = "acoustid"
	MatchSourceMBTrackID    MatchSource = "musicbrainz_track_id"
	MatchSourceMBTagLegacy  MatchSource = "musicbrainz_tag"
	MatchSourceEmbeddedTags MatchSource = "tags"
	MatchSourceFilename     MatchSource = "filename"
	MatchSourceISBNLocal    MatchSource = "isbn_local"
	MatchSourceISBNProvider MatchSource = "isbn_provider"
	MatchSourceTitle        MatchSource = "title"
)

// Label returns the short human-readable name for this match source.
func (s MatchSource) Label() string {
	switch s {
	case MatchSourceOSHash:
		return "File Hash"
	case MatchSourceProviderHash:
		return "Provider Hash"
	case MatchSourceAcoustID:
		return "Audio Fingerprint"
	case MatchSourceMBTrackID, MatchSourceMBTagLegacy:
		return "MusicBrainz Tag"
	case MatchSourceEmbeddedTags:
		return "Embedded Tags"
	case MatchSourceFilename:
		return "Filename"
	case MatchSourceISBNLocal:
		return "ISBN (Library)"
	case MatchSourceISBNProvider:
		return "ISBN (Provider)"
	case MatchSourceTitle:
		return "Title Match"
	default:
		return string(s)
	}
}

// Description returns a one-line explanation of how this source found its match.
func (s MatchSource) Description() string {
	switch s {
	case MatchSourceOSHash:
		return "File fingerprint matched an item already in your library"
	case MatchSourceProviderHash:
		return "File hash submitted to metadata provider (e.g. StashDB) and returned a match"
	case MatchSourceAcoustID:
		return "Audio fingerprint computed via AcoustID and matched on MusicBrainz"
	case MatchSourceMBTrackID, MatchSourceMBTagLegacy:
		return "MusicBrainz recording ID found in embedded file tags"
	case MatchSourceEmbeddedTags:
		return "Title and artist matched from embedded metadata tags (FLAC/ID3)"
	case MatchSourceFilename:
		return "Title parsed from the filename"
	case MatchSourceISBNLocal:
		return "ISBN matched an item already in your library"
	case MatchSourceISBNProvider:
		return "ISBN looked up against a metadata provider"
	case MatchSourceTitle:
		return "Title searched against a metadata provider"
	default:
		return ""
	}
}

// MusicMatchDetail carries the three-tier confidence evidence for a music file.
// Stored as MediaFile.MatchDetail for music content; nil for other content types.
type MusicMatchDetail struct {
	RecordingMBID       string       `json:"recording_mbid,omitempty"`
	RecordingTitle      string       `json:"recording_title,omitempty"`
	RecordingConfidence float64      `json:"recording_confidence"`
	ReleaseGroupMBID    string       `json:"release_group_mbid,omitempty"`
	ReleaseMBID         string       `json:"release_mbid,omitempty"`
	ReleaseTitle        string       `json:"release_title,omitempty"`
	ReleaseDate         string       `json:"release_date,omitempty"`
	ReleaseLabel        string       `json:"release_label,omitempty"`
	ReleaseCountry      string       `json:"release_country,omitempty"`
	ReleaseCatalog      string       `json:"release_catalog,omitempty"`
	ReleaseBarcode      string       `json:"release_barcode,omitempty"`
	ReleaseConfidence   float64      `json:"release_confidence"`
	MatchReasons        MatchReasons `json:"match_reasons"`
}

// MatchReasons records the individual signal contributions to a match score.
// All values are 0.0–1.0; zero means the signal was absent or did not fire.
type MatchReasons struct {
	Fingerprint  float64 `json:"fingerprint,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	TitleTag     float64 `json:"title_tag,omitempty"`
	ArtistTag    float64 `json:"artist_tag,omitempty"`
	AlbumTag     float64 `json:"album_tag,omitempty"`
	AlbumContext float64 `json:"album_context,omitempty"`
}

// MatchCandidate is a potential match for a scanned file. Item is the local
// library record when one exists; ExternalItem carries provider metadata when
// the source identified the file but no local item has been created yet.
// Both may be set (local item found via external lookup); Item may be nil
// when the provider recognised the file but the title is not in the library.
type MatchCandidate struct {
	Item                *Item
	ExternalItem        *ExternalItem
	Confidence          float64           // combined; drives auto-import threshold
	RecordingConfidence float64           // how sure we are WHAT this recording is
	ReleaseConfidence   float64           // how sure we are WHICH edition
	Source              string            // strategy: "oshash", "acoustid", "isbn_provider", "filename", etc.
	MusicDetail         *MusicMatchDetail // nil for non-music content
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
