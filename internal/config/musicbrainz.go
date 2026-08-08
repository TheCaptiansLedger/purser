package config

import "time"

// MusicBrainz configures the MusicBrainz adapter
// (internal/adapters/musicbrainz) at the level an operator plausibly needs
// to override — everything else (HTTP client tuning, cache TTL) keeps that
// package's own sane defaults, translated at the composition root
// (cmd/purser), the same "config stays adapter-agnostic" convention
// Database/BadgerConfig/SQLConfig already follow. See
// docs/adr/0010-configuration.md.
//
// ResponseHeaderTimeout is the one HTTP-client knob that earns an
// exception to that convention: a real, uncached MusicBrainz
// release/artist lookup with a heavy inc= (recordings+labels+release-
// groups) can legitimately take several seconds longer than
// pkg/httpclient's generic 10s default on a cold request — observed
// directly against musicbrainz.org during M11b's manual verification
// (purser#522), not a hypothetical. musicbrainz.DefaultConfig() already
// raises its own baked-in default above pkg/httpclient's generic one for
// this reason; this field exists so an operator hitting it worse than
// that default can raise it further without a code change.
type MusicBrainz struct {
	// BaseURL overrides the MusicBrainz API root — e.g. for a self-hosted
	// mirror. Empty means "use internal/adapters/musicbrainz's own
	// default".
	BaseURL string `mapstructure:"base_url"`

	// ResponseHeaderTimeout overrides how long a single MusicBrainz
	// request waits for response headers before failing. Zero means "use
	// internal/adapters/musicbrainz's own default" (see musicbrainz.
	// DefaultConfig).
	ResponseHeaderTimeout time.Duration `mapstructure:"response_header_timeout"`
}

// DefaultMusicBrainz returns MusicBrainz's defaults: both fields left at
// their zero value so the composition root falls back to
// musicbrainz.DefaultConfig()'s own BaseURL/ResponseHeaderTimeout.
func DefaultMusicBrainz() MusicBrainz {
	return MusicBrainz{}
}
