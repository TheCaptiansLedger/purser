package ports

import "context"

// WikidataClient is the port for the single Wikidata adapter
// (docs/adr/0027-provider-independence.md). Like StashDBClient/
// ThePornDBClient/MusicBrainzClient, this is a deliberate exception to
// "ports describe capability, never a specific provider"
// (docs/adr/0001-hexagonal-architecture.md): Wikidata gets its own port,
// never shared with MusicBrainzClient — MusicBrainz only ever hands a
// caller the entity *URL* (a "wikidata" url-rel, e.g.
// "https://www.wikidata.org/wiki/Q845084" — confirmed live against REO
// Speedwagon's own relations), it never resolves that URL to a photo
// itself.
//
// LookupImage is Wikidata's real capability for #703: resolve an entity to
// whatever Commons files its P18 ("image") claim points at, already turned
// into hotlinkable Wikimedia Commons file URLs (see
// internal/adapters/wikidata's own doc comment for the filename-to-URL
// step) — a P18 claim's raw value is a bare Commons filename, not a URL,
// and nothing in this codebase needs that raw filename exposed. Order is
// whatever Wikidata's own claims array returned, never re-sorted (same
// read-only-passthrough rule ThePornDBClient.ResolveJAVCode's doc comment
// states). Returns ErrNotFound both when the entity itself doesn't exist
// and when it exists but carries no P18 claim — confirmed live, Wikidata's
// own wbgetclaims answers both cases with HTTP 200 (a "no-such-entity"
// error object for the former, an empty claims object for the latter), no
// HTTP-404 equivalent to map the way this port's REST-backed siblings do.
type WikidataClient interface {
	LookupImage(ctx context.Context, entityURL string) ([]WikidataImage, error)
}

// WikidataImage is one Commons file resolved from an entity's P18 claim.
type WikidataImage struct {
	URL string `json:"url"`
}
