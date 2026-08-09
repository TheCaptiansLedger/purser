package config

// AcoustID configures the AcoustID adapter (internal/adapters/acoustid) at
// the level an operator plausibly needs to override — see MusicBrainz's
// doc comment for the "config stays adapter-agnostic" convention this
// follows.
type AcoustID struct {
	// BaseURL overrides the AcoustID lookup endpoint. Empty means "use
	// internal/adapters/acoustid's own default".
	BaseURL string `mapstructure:"base_url"`

	// APIKey is the AcoustID client API key. Empty is a valid,
	// deliberately supported configuration — the AcoustID identification
	// step is optional corroboration (docs/adr/0025-music-identification-confidence-scoring.md),
	// gated on tag-derived signals not already resolving a group; the
	// composition root wires a no-op AcoustIDClient when this is empty
	// rather than failing startup.
	APIKey string `mapstructure:"api_key" secret:"true"`
}

// DefaultAcoustID returns AcoustID's defaults: both fields empty (no
// AcoustID adapter constructed unless an operator opts in with an API key).
func DefaultAcoustID() AcoustID {
	return AcoustID{}
}
