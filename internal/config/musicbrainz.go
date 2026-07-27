package config

// MusicBrainz configures the MusicBrainz adapter
// (internal/adapters/musicbrainz) at the level an operator plausibly needs
// to override — everything else (HTTP client tuning, cache TTL) keeps that
// package's own sane defaults, translated at the composition root
// (cmd/purser), the same "config stays adapter-agnostic" convention
// Database/BadgerConfig/SQLConfig already follow. See
// docs/adr/0010-configuration.md.
type MusicBrainz struct {
	// BaseURL overrides the MusicBrainz API root — e.g. for a self-hosted
	// mirror. Empty means "use internal/adapters/musicbrainz's own
	// default".
	BaseURL string `mapstructure:"base_url"`
}

// DefaultMusicBrainz returns MusicBrainz's defaults: BaseURL left empty so
// the composition root falls back to musicbrainz.DefaultConfig().BaseURL.
func DefaultMusicBrainz() MusicBrainz {
	return MusicBrainz{}
}
