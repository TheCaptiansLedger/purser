package config

// QBittorrent configures the qBittorrent adapter
// (internal/adapters/qbittorrent) — see
// docs/technical/acquisition-download-client.md. Like Prowlarr, qBittorrent
// is self-hosted only with no public default instance, so BaseURL is
// required with no fallback. Unlike Prowlarr's static API key, qBittorrent's
// WebUI API authenticates via a Username/Password login call that returns a
// session cookie the adapter carries on every subsequent request — see
// internal/adapters/qbittorrent's package doc comment. Lives as its own
// flat top-level Config field (qbittorrent.*), not nested under Sources —
// Sources is scoped to docs/adr/0027-provider-independence.md-style
// external metadata/image providers; qBittorrent is a download-client
// backend, a different capability
// ([0001](docs/adr/0001-hexagonal-architecture.md)'s DownloadClient port),
// the same reasoning that already keeps Prowlarr flat instead of under
// Sources.
type QBittorrent struct {
	// Enabled gates whether a consuming composition root constructs a
	// real qBittorrent client at all — torrent download submission is
	// opt-in.
	Enabled bool `mapstructure:"enabled"`

	// BaseURL is the operator's own qBittorrent WebUI API root, e.g.
	// "http://qbittorrent.local:8080". Required for Enabled to have any
	// effect: internal/adapters/qbittorrent.New errors on an empty
	// BaseURL. No default — there is no public qBittorrent instance to
	// fall back to.
	BaseURL string `mapstructure:"base_url"`

	// Username authenticates against POST /api/v2/auth/login. Required
	// for Enabled to have any effect: internal/adapters/qbittorrent.New
	// errors on an empty Username.
	Username string `mapstructure:"username"`

	// Password authenticates against POST /api/v2/auth/login. Required
	// for Enabled to have any effect: internal/adapters/qbittorrent.New
	// errors on an empty Password.
	Password string `mapstructure:"password" secret:"true"`
}

// DefaultQBittorrent returns QBittorrent's defaults: disabled, no base URL,
// no credentials.
func DefaultQBittorrent() QBittorrent {
	return QBittorrent{}
}
