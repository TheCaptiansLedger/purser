package ports

import "context"

// StashDBClient is the port for the single StashDB adapter
// (docs/adr/0027-provider-independence.md). Like MusicBrainzClient and
// AcoustIDClient, this is a deliberate exception to "ports describe
// capability, never a specific provider"
// (docs/adr/0001-hexagonal-architecture.md): StashDB gets its own port,
// never shared with any other provider — ThePornDB gets its own, separate
// port when it's built (see ADR-0027). Its methods return StashDB's own
// DTO shapes below, mirroring stash-box's GraphQL schema
// (https://github.com/stashapp/stash-box), not domain types — mapping a
// result into a domain Studio/Person/Item is the caller's job (a future
// AfterDark identifier), not this port's.
//
// Every Lookup* method returns ErrNotFound when StashDB's GraphQL query
// resolves to a null result — GraphQL has no HTTP-404 equivalent; a
// resolver returns null for "not found" the same way this port's REST
// counterparts (MusicBrainzClient) map a 404. Search methods never return
// ErrNotFound for a zero-result query — an empty slice is a valid,
// non-error result.
type StashDBClient interface {
	// LookupPerformer fetches one performer by StashDB ID.
	LookupPerformer(ctx context.Context, id string) (*Performer, error)

	// SearchPerformers free-text searches performers by name/alias.
	SearchPerformers(ctx context.Context, term string) ([]Performer, error)

	// LookupStudio fetches one studio by StashDB ID. StashDB's real
	// GraphQL schema has no free-text studio search (only findStudio, by
	// ID or name, and a paginated queryStudios filter this port doesn't
	// expose) — unlike performers/scenes, there is deliberately no
	// SearchStudios here.
	LookupStudio(ctx context.Context, id string) (*Studio, error)

	// LookupScene fetches one scene by StashDB ID.
	LookupScene(ctx context.Context, id string) (*Scene, error)

	// SearchScenes free-text searches scenes by title.
	SearchScenes(ctx context.Context, term string) ([]Scene, error)

	// FindScenesByFingerprints resolves one file's candidate fingerprints
	// (OSHash and/or PHash) to the scene(s) StashDB has recorded against
	// any of them. StashDB's underlying findScenesBySceneFingerprints
	// query is batch-shaped (many files' fingerprint sets in, one
	// matching scene list per file out) since stash-box was built for
	// batch import tools; this port flattens that to one file at a time,
	// matching this codebase's "one file is one identification unit"
	// pipeline design (docs/adr/0024-pipeline-core.md). An empty result is
	// not an error — no recorded scene shares any of the given
	// fingerprints is a valid outcome.
	FindScenesByFingerprints(ctx context.Context, fingerprints []SceneFingerprint) ([]Scene, error)
}

// FingerprintAlgorithm is one of StashDB's supported scene-fingerprint
// algorithms.
type FingerprintAlgorithm string

// The three algorithms StashDB records scene fingerprints under.
const (
	FingerprintAlgorithmMD5    FingerprintAlgorithm = "MD5"
	FingerprintAlgorithmOSHash FingerprintAlgorithm = "OSHASH"
	FingerprintAlgorithmPHash  FingerprintAlgorithm = "PHASH"
)

// SceneFingerprint is one candidate fingerprint fed into
// FindScenesByFingerprints — StashDB's own FingerprintQueryInput shape
// (hash + algorithm only; unlike Fingerprint below, StashDB's query input
// carries no duration).
type SceneFingerprint struct {
	Hash      string               `json:"hash"`
	Algorithm FingerprintAlgorithm `json:"algorithm"`
}

// Site is the external site a URL belongs to (a studio's own site, a tube
// site, a social profile, etc.) — StashDB's own Site DTO.
type Site struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// URL is one external link attached to a Performer/Studio/Scene, tagged
// with the Site it belongs to.
type URL struct {
	URL  string `json:"url"`
	Site Site   `json:"site"`
}

// Image is one image StashDB has recorded for a Performer/Studio/Scene.
type Image struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Tag is one descriptive tag attached to a Scene.
type Tag struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
}

// Performer is StashDB's performer DTO.
type Performer struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Disambiguation  string   `json:"disambiguation"`
	Aliases         []string `json:"aliases"`
	Gender          string   `json:"gender"`
	URLs            []URL    `json:"urls"`
	BirthDate       string   `json:"birth_date"`
	Ethnicity       string   `json:"ethnicity"`
	Country         string   `json:"country"`
	EyeColor        string   `json:"eye_color"`
	HairColor       string   `json:"hair_color"`
	Height          int      `json:"height"`
	CupSize         string   `json:"cup_size"`
	BandSize        int      `json:"band_size"`
	WaistSize       int      `json:"waist_size"`
	HipSize         int      `json:"hip_size"`
	BreastType      string   `json:"breast_type"`
	CareerStartYear int      `json:"career_start_year"`
	CareerEndYear   int      `json:"career_end_year"`
	Images          []Image  `json:"images"`
	IsFavorite      bool     `json:"is_favorite"`
	Deleted         bool     `json:"deleted"`
	MergedIDs       []string `json:"merged_ids"`
}

// Studio is StashDB's studio DTO. Parent/ChildStudios are only ever
// populated with ID/Name by this adapter's queries (see
// internal/adapters/stashdb's studioFields) — fetching a parent/child
// studio's own full detail is a separate LookupStudio call, avoiding
// unbounded query depth for what is, in practice, a shallow hierarchy.
type Studio struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	URLs         []URL    `json:"urls"`
	Parent       *Studio  `json:"parent,omitempty"`
	ChildStudios []Studio `json:"child_studios"`
	Images       []Image  `json:"images"`
	Deleted      bool     `json:"deleted"`
	IsFavorite   bool     `json:"is_favorite"`
}

// PerformerAppearance pairs a Performer with the (optional) alias they
// performed the Scene under.
type PerformerAppearance struct {
	Performer Performer `json:"performer"`
	As        string    `json:"as"`
}

// Fingerprint is one fingerprint StashDB has recorded against a Scene —
// the response shape (carries Duration/Submissions/UserSubmitted), distinct
// from SceneFingerprint, the query-input shape FindScenesByFingerprints
// sends.
type Fingerprint struct {
	Hash          string               `json:"hash"`
	Algorithm     FingerprintAlgorithm `json:"algorithm"`
	Duration      int                  `json:"duration"`
	Submissions   int                  `json:"submissions"`
	UserSubmitted bool                 `json:"user_submitted"`
}

// Scene is StashDB's scene DTO.
type Scene struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	Details      string                `json:"details"`
	ReleaseDate  string                `json:"release_date"`
	URLs         []URL                 `json:"urls"`
	Studio       *Studio               `json:"studio,omitempty"`
	Tags         []Tag                 `json:"tags"`
	Images       []Image               `json:"images"`
	Performers   []PerformerAppearance `json:"performers"`
	Fingerprints []Fingerprint         `json:"fingerprints"`
	Duration     int                   `json:"duration"`
	Director     string                `json:"director"`
	Code         string                `json:"code"`
	Deleted      bool                  `json:"deleted"`
}
