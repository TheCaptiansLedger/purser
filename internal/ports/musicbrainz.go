package ports

import "context"

// MusicBrainzClient is the port for the single MusicBrainz adapter
// (docs/technical/music-musicbrainz-adapter.md, ADR-0025). It is a
// deliberate, narrow exception to the "ports describe capability, never a
// specific provider" rule in docs/adr/0001-hexagonal-architecture.md:
// MusicBrainz is a singular identity graph with exactly one real
// implementation, not a swappable capability like ImageSource where
// multiple providers compete for the same role. Its methods return
// MusicBrainz's own DTO shapes below, not domain types — mapping a DTO
// into a domain.Artist/Group/MusicRelease/Item is the caller's job (the
// Music scan identifier, M7), not this port's. This keeps the port
// satisfiable by a fixture-backed fake with zero knowledge of the real
// adapter, same as any other port, even though its data shapes are
// provider-specific.
//
// Every Lookup* method returns ErrNotFound when MusicBrainz responds 404
// (unknown MBID/ISRC). Search/List methods never return ErrNotFound for a
// zero-result query — an empty slice is a valid, non-error result.
type MusicBrainzClient interface {
	// LookupArtist fetches one artist by MBID.
	LookupArtist(ctx context.Context, mbid string) (*Artist, error)

	// SearchArtists free-text searches artists by name.
	SearchArtists(ctx context.Context, query string) ([]Artist, error)

	// LookupReleaseGroup fetches one release group by MBID.
	LookupReleaseGroup(ctx context.Context, mbid string) (*ReleaseGroup, error)

	// ListReleaseGroupsForArtist lists an artist's release groups
	// (discography) by artist MBID.
	ListReleaseGroupsForArtist(ctx context.Context, artistMBID string) ([]ReleaseGroup, error)

	// SearchReleaseGroups free-text searches release groups by artist name
	// and album name, with no MBID on either side — fuzzy candidate
	// generation fed by tag-derived or filename-derived guesses.
	SearchReleaseGroups(ctx context.Context, artistName, albumName string) ([]ReleaseGroup, error)

	// LookupRelease fetches one release (a specific pressing/edition) by
	// MBID, including its full track listing.
	LookupRelease(ctx context.Context, mbid string) (*Release, error)

	// ListReleasesForReleaseGroup lists every known pressing/edition of a
	// release group by release-group MBID.
	ListReleasesForReleaseGroup(ctx context.Context, rgMBID string) ([]Release, error)

	// SearchReleaseByBarcode resolves a barcode to the release(s) carrying
	// it.
	SearchReleaseByBarcode(ctx context.Context, barcode string) ([]Release, error)

	// LookupRecordingByISRC resolves a recording ISRC to the recording(s)
	// sharing it — an ISRC can, rarely, be assigned to more than one
	// recording.
	LookupRecordingByISRC(ctx context.Context, isrc string) ([]Recording, error)
}

// Area is a MusicBrainz place (country, region, city) as embedded on an
// Artist or ReleaseEvent.
type Area struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name"`
	ISO31661Codes  []string `json:"iso-3166-1-codes"`
	Disambiguation string   `json:"disambiguation"`
}

// LifeSpan is an Artist's begin/end dates — birth/death for a person,
// formed/disbanded for a group.
type LifeSpan struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
	Ended bool   `json:"ended"`
}

// Alias is an alternate name for an Artist.
type Alias struct {
	Name     string `json:"name"`
	SortName string `json:"sort-name"`
	Locale   string `json:"locale"`
	Type     string `json:"type"`
	Primary  bool   `json:"primary"`
	Begin    string `json:"begin"`
	End      string `json:"end"`
	Ended    bool   `json:"ended"`
}

// RelationURL is the target of a Relation whose direction points at an
// external URL rather than another artist.
type RelationURL struct {
	Resource string `json:"resource"`
}

// RelationArtist is the target of a Relation whose direction points at
// another artist (e.g. a band member).
type RelationArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Relation is one MusicBrainz relationship edge — e.g. "member of band",
// "official homepage", "wikipedia" — embedded on an Artist. Begin/End/Ended
// carry the relationship's own life span (e.g. when a member joined/left a
// band), distinct from the target Artist's own LifeSpan. Attributes is
// relation-type-specific detail MusicBrainz attaches to some edges — for
// "member of band" it's the instrument/role list (e.g. "vocal", "guitar").
type Relation struct {
	Type       string          `json:"type"`
	Direction  string          `json:"direction"`
	URL        *RelationURL    `json:"url,omitempty"`
	Artist     *RelationArtist `json:"artist,omitempty"`
	Begin      string          `json:"begin"`
	End        string          `json:"end"`
	Ended      bool            `json:"ended"`
	Attributes []string        `json:"attributes"`
}

// Artist is MusicBrainz's artist DTO — the identity anchor for a person,
// group, orchestra, choir, character, or other credited entity.
type Artist struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	SortName       string     `json:"sort-name"`
	Disambiguation string     `json:"disambiguation"`
	Type           string     `json:"type"`
	Country        string     `json:"country"`
	Area           *Area      `json:"area,omitempty"`
	BeginArea      *Area      `json:"begin-area,omitempty"`
	LifeSpan       LifeSpan   `json:"life-span"`
	ISNIs          []string   `json:"isnis"`
	IPIs           []string   `json:"ipis"`
	Gender         string     `json:"gender"`
	GenderID       string     `json:"gender-id"`
	Aliases        []Alias    `json:"aliases"`
	Relations      []Relation `json:"relations"`
}

// ReleaseGroup is MusicBrainz's release-group DTO — the conceptual album,
// independent of any specific pressing/edition.
type ReleaseGroup struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Disambiguation   string   `json:"disambiguation"`
	PrimaryType      string   `json:"primary-type"`
	SecondaryTypes   []string `json:"secondary-types"`
	FirstReleaseDate string   `json:"first-release-date"`
}

// Label is a record label as embedded in a Release's LabelInfo.
type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LabelInfo pairs a Label with its catalog number for one Release.
type LabelInfo struct {
	Label         Label  `json:"label"`
	CatalogNumber string `json:"catalog-number"`
}

// ReleaseEvent is one place/date a Release was issued.
type ReleaseEvent struct {
	Date string `json:"date"`
	Area *Area  `json:"area,omitempty"`
}

// CoverArtArchive reports whether the Cover Art Archive actually has
// artwork for a Release, returned inline by MusicBrainz so callers can
// check existence before deriving a Cover Art Archive URL.
type CoverArtArchive struct {
	Artwork  bool `json:"artwork"`
	Front    bool `json:"front"`
	Back     bool `json:"back"`
	Darkened bool `json:"darkened"`
	Count    int  `json:"count"`
}

// ArtistCredit is one artist's credited contribution to a Recording or
// Release (supports multi-artist credits, e.g. "A feat. B").
type ArtistCredit struct {
	Name   string         `json:"name"`
	Artist RelationArtist `json:"artist"`
}

// Track is one entry in a Medium's track listing.
type Track struct {
	ID        string     `json:"id"`
	Position  int        `json:"position"`
	Number    string     `json:"number"`
	Title     string     `json:"title"`
	Length    int        `json:"length"`
	Recording *Recording `json:"recording,omitempty"`
}

// Medium is one disc/side of a Release (media[] in MusicBrainz's shape).
type Medium struct {
	ID          string  `json:"id"`
	Position    int     `json:"position"`
	Format      string  `json:"format"`
	TrackCount  int     `json:"track-count"`
	TrackOffset int     `json:"track-offset"`
	Tracks      []Track `json:"tracks"`
}

// Release is MusicBrainz's release DTO — one specific pressing/edition of
// a ReleaseGroup, with its full track listing when fetched via
// LookupRelease.
type Release struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Status          string          `json:"status"`
	Country         string          `json:"country"`
	Date            string          `json:"date"`
	Barcode         string          `json:"barcode"`
	ASIN            string          `json:"asin"`
	Disambiguation  string          `json:"disambiguation"`
	Packaging       string          `json:"packaging"`
	Quality         string          `json:"quality"`
	LabelInfo       []LabelInfo     `json:"label-info"`
	ReleaseEvents   []ReleaseEvent  `json:"release-events"`
	CoverArtArchive CoverArtArchive `json:"cover-art-archive"`
	Media           []Medium        `json:"media"`
	ReleaseGroup    *ReleaseGroup   `json:"release-group,omitempty"`
	ArtistCredit    []ArtistCredit  `json:"artist-credit,omitempty"`
}

// Recording is MusicBrainz's recording DTO — a specific studio/live
// performance, distinct from any Track entry that references it.
type Recording struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Length           int            `json:"length"`
	Disambiguation   string         `json:"disambiguation"`
	Video            bool           `json:"video"`
	FirstReleaseDate string         `json:"first-release-date"`
	ISRCs            []string       `json:"isrcs,omitempty"`
	ArtistCredit     []ArtistCredit `json:"artist-credit,omitempty"`
	Releases         []Release      `json:"releases,omitempty"`
}
