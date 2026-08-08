package ports

import (
	"context"
	"encoding/json"
)

// ThePornDBClient is the port for the single ThePornDB adapter
// (docs/adr/0027-provider-independence.md). Like StashDBClient,
// MusicBrainzClient, and AcoustIDClient, this is a deliberate exception to
// "ports describe capability, never a specific provider"
// (docs/adr/0001-hexagonal-architecture.md): ThePornDB gets its own port,
// never shared with StashDBClient — the two are direct competitors covering
// the same ground (performers, scenes), and per ADR-0027 the server never
// picks a winner between them. Its methods return ThePornDB's own DTO
// shapes below, mirroring api.theporndb.net's REST schema (live-verified
// against the real API during AD2's implementation, not guessed — see
// docs/technical/afterdark-data_model.md Section 2 for the original
// research pass, extended here with direct verification of lookup/hash/JAV
// endpoints that pass didn't confirm), not domain types — mapping a result
// into a domain Studio/Person/Item is the caller's job (a future AfterDark
// identifier), not this port's.
//
// Every Lookup* method returns ErrNotFound when ThePornDB answers HTTP 404
// (confirmed live: {"message": "performer not found"} /
// {"message": "scene not found"} / {"message": "hash not found"}). Search
// methods never return ErrNotFound for a zero-result query — an empty slice
// is a valid, non-error result (also confirmed live).
type ThePornDBClient interface {
	// LookupPerformer fetches one performer by ThePornDB UUID.
	LookupPerformer(ctx context.Context, id string) (*TPDBPerformer, error)

	// SearchPerformers free-text searches performers by name/alias.
	SearchPerformers(ctx context.Context, term string) ([]TPDBPerformer, error)

	// LookupScene fetches one scene (or JAV title — both share this same
	// resource shape, distinguished only by Scene.Type) by ThePornDB UUID.
	LookupScene(ctx context.Context, id string) (*TPDBScene, error)

	// SearchScenes free-text searches scenes by title.
	SearchScenes(ctx context.Context, term string) ([]TPDBScene, error)

	// LookupSceneByHash resolves one OSHash/PHash value to the scene
	// ThePornDB has recorded it against, via the dedicated
	// GET /scenes/hash/{hash} endpoint (confirmed live — distinct from the
	// general GET /scenes?hash= search, which returns a list; this port
	// exposes the single-resource lookup since exactly one scene owns a
	// given hash, mirroring FindScenesByFingerprints' one-hash-in shape but
	// singular here since ThePornDB's hash endpoint is itself singular,
	// unlike StashDB's batch-shaped query).
	LookupSceneByHash(ctx context.Context, hash string) (*TPDBScene, error)

	// ResolveJAVCode resolves a JAV product code (e.g. "SSIS-001") via
	// GET /jav?parse={code}. Confirmed live: this is NOT a single-match
	// resolver — it returns a ranked list of candidate scenes (ThePornDB's
	// own relevance ordering, best match first), ordinarily including
	// scenes that don't share the exact code, since the endpoint is built
	// to tolerate an imperfectly parsed filename. This port returns that
	// list exactly as ThePornDB ordered it — never re-sorted or filtered —
	// per ADR-0027's read-only-passthrough rule; picking among candidates
	// is the caller's job. An empty slice (not an error) means no
	// candidates at all.
	ResolveJAVCode(ctx context.Context, code string) ([]TPDBScene, error)
}

// TPDBImage is one performer poster image ThePornDB has recorded, with its
// own numeric ID (ThePornDB image IDs, unlike StashDB's Image, are plain
// integers) and display Order.
type TPDBImage struct {
	ID    int    `json:"id"`
	URL   string `json:"url"`
	Size  int    `json:"size"`
	Order int    `json:"order"`
}

// TPDBPosters is the four pre-cropped poster sizes ThePornDB serves for a
// Scene — confirmed live on both the search and single-lookup shapes.
type TPDBPosters struct {
	Full   string `json:"full"`
	Large  string `json:"large"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
}

// TPDBPerformerExtras is the nested biographical/physical-attribute bag
// every Performer carries under its "extras" key — field names are
// ThePornDB's own (confirmed live against a real performer lookup), closer
// to free text than StashDB's typed enums, per
// docs/technical/afterdark-data_model.md's Section 3 comparison.
type TPDBPerformerExtras struct {
	Gender            string `json:"gender"`
	Birthday          string `json:"birthday"`
	BirthdayTimestamp int64  `json:"birthday_timestamp"`
	Birthplace        string `json:"birthplace"`
	BirthplaceCode    string `json:"birthplace_code"`
	Astrology         string `json:"astrology"`
	Ethnicity         string `json:"ethnicity"`
	Nationality       string `json:"nationality"`
	HairColour        string `json:"hair_colour"`
	EyeColour         string `json:"eye_colour"`
	Weight            string `json:"weight"`
	Height            string `json:"height"`
	Measurements      string `json:"measurements"`
	CupSize           string `json:"cupsize"`
	Tattoos           string `json:"tattoos"`
	Piercings         string `json:"piercings"`
	Waist             string `json:"waist"`
	Hips              string `json:"hips"`
	FakeBoobs         bool   `json:"fake_boobs"`
	SameSexOnly       bool   `json:"same_sex_only"`
	CareerStartYear   int    `json:"career_start_year"`
	CareerEndYear     int    `json:"career_end_year"`
	// Links is a flat map keyed by external site display name ("IAFD",
	// "StashDB", "Pornhub", ...) to that site's profile URL for this
	// performer — confirmed live, including a direct "StashDB" key
	// cross-referencing the same performer's stashdb.org page (see
	// docs/technical/afterdark-data_model.md Section 2's cross-linking
	// note). Same undifferentiated-by-type shape as StashDB's urls[], but
	// as a map instead of a list.
	Links TPDBLinks `json:"links"`
}

// TPDBLinks is TPDBPerformerExtras.Links' map[string]string, with a custom
// decoder tolerating both JSON shapes the live API actually sends: a real
// object ({"IAFD": "https://...", ...}) when links exist, or a bare empty
// array ([]) when they don't — confirmed live (a caching PHP/Laravel
// backend serializes an empty associative array as JSON "[]", not "{}";
// this drift wasn't visible in docs/technical/afterdark-data_model.md's
// original research pass, and only surfaced running live_test.go against
// real search results during this adapter's implementation).
type TPDBLinks map[string]string

// UnmarshalJSON implements json.Unmarshaler.
func (l *TPDBLinks) UnmarshalJSON(data []byte) error {
	if string(data) == "[]" || string(data) == "null" {
		*l = nil
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*l = m
	return nil
}

// TPDBPerformer is ThePornDB's performer DTO. A canonical performer has
// IsParent=true and a nil Parent. A site-specific persona (confirmed live:
// a scene's Performers[] entries return this shape with IsParent=false)
// carries a populated Parent pointing back to the canonical record — see
// the port's doc comment and docs/technical/afterdark-data_model.md's
// "Performer alias/parent linking" note. SitePerformers (a further, deeper
// per-site breakdown returned only by LookupPerformer) is deliberately not
// mapped here — out of scope for AD2, same "deliberately shallow" tradeoff
// stashdb.go's Studio.Parent/ChildStudios already makes.
type TPDBPerformer struct {
	ID             string              `json:"id"`
	LegacyID       int                 `json:"_id"`
	Slug           string              `json:"slug"`
	Name           string              `json:"name"`
	FullName       string              `json:"full_name"`
	Disambiguation string              `json:"disambiguation"`
	Bio            string              `json:"bio"`
	Rating         float64             `json:"rating"`
	IsParent       bool                `json:"is_parent"`
	Image          string              `json:"image"`
	Thumbnail      string              `json:"thumbnail"`
	Face           string              `json:"face"`
	Posters        []TPDBImage         `json:"posters"`
	Aliases        []string            `json:"aliases"`
	Extras         TPDBPerformerExtras `json:"extras"`
	Parent         *TPDBPerformer      `json:"parent,omitempty"`
}

// TPDBSite is the Site/Network/Studio hierarchy object nested in a Scene —
// ThePornDB's own Site DTO. Parent/Network are deliberately shallow
// (one level, self-referential) — same tradeoff as stashdb.go's Studio.
type TPDBSite struct {
	UUID      string    `json:"uuid"`
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	ShortName string    `json:"short_name"`
	URL       string    `json:"url"`
	Rating    float64   `json:"rating"`
	Logo      string    `json:"logo"`
	Favicon   string    `json:"favicon"`
	Poster    string    `json:"poster"`
	Network   *TPDBSite `json:"network,omitempty"`
	Parent    *TPDBSite `json:"parent,omitempty"`
}

// TPDBTag is one descriptive tag attached to a Scene — confirmed live as
// {id, uuid, name}, no description/aliases the way StashDB's Tag carries.
type TPDBTag struct {
	ID   int    `json:"id"`
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// TPDBDirector is one director credited on a Scene — confirmed live as
// {id, uuid, name, slug}, its own shape distinct from TPDBTag.
type TPDBDirector struct {
	ID   int    `json:"id"`
	UUID string `json:"uuid"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// TPDBHash is one fingerprint ThePornDB has recorded against a Scene —
// mirrors StashDB's Fingerprint almost field-for-field (Type instead of
// Algorithm). Users (the list of submitter account IDs) is deliberately not
// mapped — internal to ThePornDB's crowd-sourcing workflow, not useful to
// this codebase, same omission stashdb.go's Fingerprint makes.
type TPDBHash struct {
	ID          int    `json:"id"`
	Hash        string `json:"hash"`
	Type        string `json:"type"`
	Duration    int    `json:"duration"`
	Submissions int    `json:"submissions"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// TPDBScene is ThePornDB's scene DTO — also the JAV-title shape (Type ==
// "JAV" on the same resource, confirmed live via GET /jav?parse=, per
// docs/technical/afterdark-data_model.md Section 2). SKU carries the raw
// JAV product code, ExternalID the normalized/hyphenated slug — both
// populated on JAV entries, ExternalID only (SKU empty) on most Western
// scenes, confirmed live.
type TPDBScene struct {
	ID          string          `json:"id"`
	LegacyID    int             `json:"_id"`
	Title       string          `json:"title"`
	Type        string          `json:"type"`
	Slug        string          `json:"slug"`
	ExternalID  string          `json:"external_id"`
	SKU         string          `json:"sku"`
	Description string          `json:"description"`
	Rating      float64         `json:"rating"`
	Date        string          `json:"date"`
	URL         string          `json:"url"`
	Duration    int             `json:"duration"`
	Image       string          `json:"image"`
	Poster      string          `json:"poster"`
	Posters     TPDBPosters     `json:"posters"`
	Site        *TPDBSite       `json:"site,omitempty"`
	Performers  []TPDBPerformer `json:"performers"`
	Tags        []TPDBTag       `json:"tags"`
	Directors   []TPDBDirector  `json:"directors"`
	Hashes      []TPDBHash      `json:"hashes"`
}
