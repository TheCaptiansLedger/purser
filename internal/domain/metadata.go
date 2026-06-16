package domain

import "time"

// ExternalItem is scene/movie/episode/track metadata returned from an external source.
// It is a data-transfer type used by MetadataSource adapters and is not persisted directly.
type ExternalItem struct {
	Source      ExternalIDSource
	ExternalID  string
	ContentType ContentType
	Title       string
	Overview    string
	Date        time.Time
	RuntimeSecs int
	ImageURL    string
	Studio      *ExternalStudio
	People      []*ExternalPerson
	Tags        []string
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
