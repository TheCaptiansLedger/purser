package config

// FanartTV configures the fanart.tv adapter (internal/adapters/fanarttv)
// at the level an operator plausibly needs to override — everything else
// (HTTP client tuning, cache TTL, the API root itself) keeps that
// package's own sane defaults, the same "config stays adapter-agnostic"
// convention StashDB/ThePornDB already follow (see config.StashDB's own
// doc comment). Like StashDB/ThePornDB, fanart.tv has no realistic
// self-hosted-mirror use case, so BaseURL is deliberately not exposed
// here — see docs/adr/0027-provider-independence.md.
type FanartTV struct {
	// Enabled gates whether a consuming composition root constructs a
	// real fanart.tv client at all — fanart.tv is an opt-in Music image
	// source, not a required one.
	Enabled bool `mapstructure:"enabled"`

	// APIKey is sent as the "api_key" query param on every fanart.tv
	// request. Required for Enabled to have any effect:
	// internal/adapters/fanarttv.New errors on an empty APIKey.
	APIKey string `mapstructure:"api_key" secret:"true"`
}

// DefaultFanartTV returns FanartTV's defaults: disabled, no API key.
func DefaultFanartTV() FanartTV {
	return FanartTV{}
}
