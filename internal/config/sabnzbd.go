package config

// SABnzbd configures the SABnzbd adapter (internal/adapters/sabnzbd) — see
// docs/technical/acquisition-download-client.md. Like Prowlarr, SABnzbd is
// self-hosted only with no public default instance, so BaseURL is required
// with no fallback. Like Prowlarr's static API key (and unlike
// qBittorrent's Username/Password session-cookie login), SABnzbd
// authenticates every call with a static APIKey carried as an "&apikey=..."
// query parameter, no login step. Lives as its own flat top-level Config
// field (sabnzbd.*), not nested under Sources — Sources is scoped to
// docs/adr/0027-provider-independence.md-style external metadata/image
// providers; SABnzbd is a download-client backend, a different capability
// ([0001](docs/adr/0001-hexagonal-architecture.md)'s DownloadClient port),
// the same reasoning that already keeps Prowlarr and QBittorrent flat
// instead of under Sources.
type SABnzbd struct {
	// Enabled gates whether a consuming composition root constructs a real
	// SABnzbd client at all — usenet download submission is opt-in.
	Enabled bool `mapstructure:"enabled"`

	// BaseURL is the operator's own SABnzbd API root, e.g.
	// "http://sabnzbd.local:8080/sabnzbd". Required for Enabled to have any
	// effect: internal/adapters/sabnzbd.New errors on an empty BaseURL. No
	// default — there is no public SABnzbd instance to fall back to.
	BaseURL string `mapstructure:"base_url"`

	// APIKey is sent as the "apikey" query parameter on every SABnzbd
	// request. Required for Enabled to have any effect:
	// internal/adapters/sabnzbd.New errors on an empty APIKey.
	APIKey string `mapstructure:"api_key"`
}

// DefaultSABnzbd returns SABnzbd's defaults: disabled, no base URL, no API
// key.
func DefaultSABnzbd() SABnzbd {
	return SABnzbd{}
}
