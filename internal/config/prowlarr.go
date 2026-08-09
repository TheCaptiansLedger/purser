package config

import "time"

// Prowlarr configures the Prowlarr adapter (internal/adapters/prowlarr) —
// see docs/technical/acquisition-indexer-search.md. Unlike MusicBrainz/
// AcoustID, Prowlarr has no public default instance: every deployment runs
// its own, so BaseURL is required with no fallback, the same
// "no realistic default" situation config.StashDB's APIKey is in, just for
// a different field. Lives as its own flat top-level Config field
// (prowlarr.*), not nested under Sources — Sources is scoped to
// docs/adr/0027-provider-independence.md-style external metadata/image
// providers; Prowlarr is an indexer-aggregator backend, a different
// capability ([0001](docs/adr/0001-hexagonal-architecture.md)'s
// IndexerSearcher port), the same reasoning that already keeps MusicBrainz/
// AcoustID flat instead of under Sources.
type Prowlarr struct {
	// Enabled gates whether a consuming composition root constructs a
	// real Prowlarr client at all — indexer search is opt-in.
	Enabled bool `mapstructure:"enabled"`

	// BaseURL is the operator's own Prowlarr API root, e.g.
	// "http://prowlarr.local:9696/api/v1". Required for Enabled to have
	// any effect: internal/adapters/prowlarr.New errors on an empty
	// BaseURL. No default — there is no public Prowlarr instance to fall
	// back to.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is sent as the "X-Api-Key" header on every Prowlarr request.
	// Required for Enabled to have any effect: internal/adapters/prowlarr.New
	// errors on an empty APIKey.
	APIKey string `mapstructure:"api_key" secret:"true"`

	// ResponseHeaderTimeout overrides how long a single Prowlarr search
	// waits for response headers before failing. Zero means "use
	// internal/adapters/prowlarr's own default" (25s — see prowlarr.
	// DefaultConfig's doc comment: a real search fans out to every
	// enabled indexer and waits for the slowest one, confirmed to take
	// 9.3s+ against a real multi-indexer instance during this adapter's
	// implementation). Same escape hatch config.MusicBrainz.
	// ResponseHeaderTimeout provides for the identical situation.
	ResponseHeaderTimeout time.Duration `mapstructure:"response_header_timeout"`
}

// DefaultProwlarr returns Prowlarr's defaults: disabled, no base URL, no
// API key, ResponseHeaderTimeout left at zero so a consuming composition
// root falls back to prowlarr.DefaultConfig()'s own value.
func DefaultProwlarr() Prowlarr {
	return Prowlarr{}
}
