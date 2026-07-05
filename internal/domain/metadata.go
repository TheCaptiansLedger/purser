package domain

import "time"

// ExternalItem is scene/movie/episode/track metadata returned from an external source.
// It is a data-transfer type used by MetadataSource adapters and is not persisted directly.
type ExternalItem struct {
	Source          ExternalIDSource
	ExternalID      string
	ContentType     ContentType
	Title           string
	Overview        string
	Sequence        string // track number, episode number, or other position indicator
	Year            int
	Date            time.Time
	RuntimeSecs     int
	ImageURL        string
	GroupExternalID string // parent release group MBID, season ID, etc.
	GroupTitle      string // title of the parent group when known from the external source
	Studio          *ExternalStudio
	People          []*ExternalPerson
	Tags            []string
	Genres          []string
	ExternalIDs     map[string]string
	Images          []ExternalImage
	ReleaseDetail   *ExternalReleaseDetail // non-nil only for music items; holds edition-specific fields
}

// ExternalReleaseDetail carries the edition-specific fields for a music recording.
// It is populated by the MBZ adapter when fetching a recording by ID and carries the
// specific release (pressing) details that differ between editions of the same conceptual album.
type ExternalReleaseDetail struct {
	ReleaseMBID    string // MBZ release MBID (a specific pressing, not the release group)
	ReleaseTitle   string // edition-qualified title, e.g. "Hi Infidelity (2024 Remaster)"
	ReleaseDate    string // ISO 8601 date string as returned by MBZ (may be partial, e.g. "1981-02")
	ReleaseLabel   string // first label name from label-info
	ReleaseCountry string // ISO 3166-1 alpha-2 country code
	ReleaseCatalog string // catalog number from the first label-info entry
	ReleaseBarcode string // barcode (EAN/UPC) as a string
}

// ExternalImage is image metadata returned by a metadata source for a given entity.
// Source is empty when returned by a provider; it is stamped by MergeExternalItems
// from the SourcedExternalItem wrapper so the merged result carries its origin.
type ExternalImage struct {
	Type   ImageType
	URL    string
	Width  int
	Height int
	Source string
}

// SourcedExternalItem pairs an ExternalItem with the name of the provider that
// produced it. The aggregator constructs this slice before calling MergeExternalItems,
// ordered by provider priority (index 0 = primary/highest priority).
type SourcedExternalItem struct {
	Source string // provider name: "musicbrainz", "fanart", "lastfm", etc.
	Item   *ExternalItem
}

// ExternalStudio carries studio or network data returned by a metadata source.
// ParentID/ParentName are the source's identifiers for the parent network; both
// may be empty if the studio has no parent or the source doesn't model one.
type ExternalStudio struct {
	Source           ExternalIDSource
	ExternalID       string
	Name             string
	Overview         string
	ImageURL         string
	WebsiteURL       string
	ParentID         string // parent's ID within the same source
	ParentName       string
	ParentImageURL   string
	ParentWebsiteURL string
}

// ExternalPerson carries performer, artist, author, or cast data returned by a
// metadata source. Metadata holds source-specific enrichment fields (birthdate,
// measurements, etc.) using the same keys that domain.Person.Metadata uses.
type ExternalPerson struct {
	Source     ExternalIDSource
	ExternalID string
	Name       string
	Aliases    []string
	Overview   string
	ImageURL   string
	Role       PersonRole     // suggested role based on source context
	Metadata   map[string]any // source-specific: birthdate, hair_color, measurements, etc.
}

// ExternalGroup carries season, album, or series data returned by a metadata
// source. It is the intermediate level between a library entry and its items —
// e.g. a TV season between a show and its episodes, or an album between an
// artist and its tracks.
type ExternalGroup struct {
	Source         ExternalIDSource
	ExternalID     string
	Title          string
	Number         int // season 1, disc 2, volume 3, etc.
	Year           int
	Overview       string
	ImageURL       string
	PrimaryType    string   // source-specific primary classification (e.g. "Album", "Single")
	SecondaryTypes []string // source-specific secondary classifications (e.g. ["Live"], ["Compilation"])
}

// AlbumFilterToken returns the Purser album section identifier for this external group.
// Values: "ep", "single", "studio", "live", "compilation", "other".
func (eg *ExternalGroup) AlbumFilterToken() string {
	switch eg.PrimaryType {
	case "EP":
		return "ep"
	case "Single":
		return "single"
	case "Album":
		for _, s := range eg.SecondaryTypes {
			switch s {
			case "Live":
				return "live"
			case "Compilation":
				return "compilation"
			}
		}
		return "studio"
	default:
		return "other"
	}
}
