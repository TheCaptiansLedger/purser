package config

// Wikidata configures the Wikidata adapter (internal/adapters/wikidata) at
// the level an operator plausibly needs to override — everything else
// (HTTP client tuning, cache TTL, the API root itself) keeps that
// package's own sane defaults, same "config stays adapter-agnostic"
// convention FanartTV/StashDB/ThePornDB already follow. Unlike those
// three, there is no APIKey field: Wikidata's action API requires no
// authentication.
type Wikidata struct {
	// Enabled gates whether a consuming composition root constructs a
	// real Wikidata client at all — Wikidata is an opt-in Person-photo
	// source, not a required one.
	Enabled bool `mapstructure:"enabled"`
}

// DefaultWikidata returns Wikidata's defaults: disabled.
func DefaultWikidata() Wikidata {
	return Wikidata{}
}
