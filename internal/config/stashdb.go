package config

// StashDB configures the StashDB adapter (internal/adapters/stashdb) at
// the level an operator plausibly needs to override — everything else
// (HTTP client tuning, cache TTL, the GraphQL endpoint itself) keeps that
// package's own sane defaults, the same "config stays adapter-agnostic"
// convention MusicBrainz/AcoustID already follow (see MusicBrainz's own
// doc comment). Unlike MusicBrainz/AcoustID, StashDB has no realistic
// self-hosted-mirror use case, so BaseURL is deliberately not exposed
// here — see docs/adr/0027-provider-independence.md.
type StashDB struct {
	// Enabled gates whether a consuming composition root constructs a
	// real StashDB client at all — StashDB is an opt-in AfterDark data
	// source, not a required one.
	Enabled bool `mapstructure:"enabled"`

	// APIKey is sent as the "ApiKey" header on every StashDB request.
	// Required for Enabled to have any effect: internal/adapters/stashdb.New
	// errors on an empty APIKey.
	APIKey string `mapstructure:"api_key" secret:"true"`
}

// DefaultStashDB returns StashDB's defaults: disabled, no API key.
func DefaultStashDB() StashDB {
	return StashDB{}
}
