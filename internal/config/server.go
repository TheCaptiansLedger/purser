package config

// Server configures the Connect/gRPC listener. See
// docs/adr/0010-configuration.md and docs/adr/0011-api-design.md.
type Server struct {
	// ListenAddr is the address the Connect/gRPC server binds, e.g. ":8080".
	ListenAddr string `mapstructure:"listen_addr"`
}

// DefaultServer returns Server's sane defaults.
func DefaultServer() Server {
	return Server{ListenAddr: ":8080"}
}
