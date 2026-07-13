package config

// Server configures the Connect/gRPC listener. See
// docs/adr/0010-configuration.md and docs/adr/0011-api-design.md.
type Server struct {
	// ListenAddr is the address the Connect/gRPC server binds, e.g. ":7474".
	ListenAddr string `mapstructure:"listen_addr"`
}

// DefaultServer returns Server's sane defaults. :7474 matches
// ops/purser.yaml, ops/compose.yml's port mapping, and every k6 test's
// fallback target — one canonical port for native and containerized runs
// alike, rather than a generic placeholder no other file agrees with.
func DefaultServer() Server {
	return Server{ListenAddr: ":7474"}
}
