package config

// ThePornDB configures the ThePornDB adapter (internal/adapters/theporndb)
// at the level an operator plausibly needs to override — everything else
// (HTTP client tuning, cache TTL, the API root itself) keeps that package's
// own sane defaults, the same "config stays adapter-agnostic" convention
// StashDB/MusicBrainz already follow (see config.StashDB's own doc
// comment). Like StashDB, ThePornDB has no realistic self-hosted-mirror use
// case, so BaseURL is deliberately not exposed here — see
// docs/adr/0027-provider-independence.md.
type ThePornDB struct {
	// Enabled gates whether a consuming composition root constructs a
	// real ThePornDB client at all — ThePornDB is an opt-in AfterDark data
	// source, not a required one.
	Enabled bool `mapstructure:"enabled"`

	// APIKey is sent as "Authorization: Bearer {APIKey}" on every ThePornDB
	// request. Required for Enabled to have any effect:
	// internal/adapters/theporndb.New errors on an empty APIKey.
	APIKey string `mapstructure:"api_key"`
}

// DefaultThePornDB returns ThePornDB's defaults: disabled, no API key.
func DefaultThePornDB() ThePornDB {
	return ThePornDB{}
}
