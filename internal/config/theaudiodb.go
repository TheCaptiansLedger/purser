package config

// TheAudioDB configures the TheAudioDB adapter
// (internal/adapters/theaudiodb) at the level an operator plausibly needs
// to override — everything else (HTTP client tuning, cache TTL, the API
// root itself) keeps that package's own sane defaults, the same
// "config stays adapter-agnostic" convention StashDB/ThePornDB already
// follow (see config.StashDB's own doc comment). Like StashDB/ThePornDB,
// TheAudioDB has no realistic self-hosted-mirror use case, so BaseURL is
// deliberately not exposed here — see
// docs/adr/0027-provider-independence.md.
type TheAudioDB struct {
	// Enabled gates whether a consuming composition root constructs a
	// real TheAudioDB client at all — TheAudioDB is an opt-in Music image
	// source, not a required one.
	Enabled bool `mapstructure:"enabled"`

	// APIKey is appended as a URL path segment on every TheAudioDB
	// request. Required for Enabled to have any effect:
	// internal/adapters/theaudiodb.New errors on an empty APIKey. The
	// free tier's key "123" works (confirmed live).
	APIKey string `mapstructure:"api_key"`
}

// DefaultTheAudioDB returns TheAudioDB's defaults: disabled, no API key.
func DefaultTheAudioDB() TheAudioDB {
	return TheAudioDB{}
}
