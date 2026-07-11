package httpclient

import "time"

// Config configures a Client built by New. Every field has a sane, secure
// default via DefaultConfig — callers only need to override what differs
// for their use case. Field names/tags follow the PURSER_<NESTED>_<KEY>
// convention documented in ADR 0010 for future Viper embedding.
//
// There is deliberately no option to disable TLS certificate verification —
// per the "as secure as possible" requirement, that footgun isn't exposed.
type Config struct {
	// Timeout bounds the entire request (dial + TLS + write + read),
	// per net/http.Client.Timeout semantics.
	Timeout time.Duration `mapstructure:"timeout"`

	// DialTimeout bounds establishing the TCP connection.
	DialTimeout time.Duration `mapstructure:"dial_timeout"`

	// KeepAlive is the TCP keep-alive interval for open connections.
	KeepAlive time.Duration `mapstructure:"keep_alive"`

	// TLSHandshakeTimeout bounds the TLS handshake after connecting.
	TLSHandshakeTimeout time.Duration `mapstructure:"tls_handshake_timeout"`

	// ResponseHeaderTimeout bounds waiting for response headers after the
	// request has been fully written.
	ResponseHeaderTimeout time.Duration `mapstructure:"response_header_timeout"`

	// ExpectContinueTimeout bounds waiting for a 100-continue response
	// when a request has an "Expect: 100-continue" header.
	ExpectContinueTimeout time.Duration `mapstructure:"expect_continue_timeout"`

	// IdleConnTimeout bounds how long an idle keep-alive connection stays
	// in the pool before being closed.
	IdleConnTimeout time.Duration `mapstructure:"idle_conn_timeout"`

	// MaxIdleConns is the maximum idle connections across all hosts.
	MaxIdleConns int `mapstructure:"max_idle_conns"`

	// MaxIdleConnsPerHost is the maximum idle connections per host.
	MaxIdleConnsPerHost int `mapstructure:"max_idle_conns_per_host"`

	// MaxConnsPerHost caps total (idle + active) connections per host.
	// Zero means unlimited.
	MaxConnsPerHost int `mapstructure:"max_conns_per_host"`

	// MaxRedirects caps how many redirects a single request follows before
	// the client gives up with an error. Zero means no redirects are
	// followed at all.
	MaxRedirects int `mapstructure:"max_redirects"`

	// UserAgent is sent on every request that doesn't already set its own
	// User-Agent header. Empty means the Go default is left untouched.
	UserAgent string `mapstructure:"user_agent"`
}

// DefaultConfig returns the sane, secure defaults every Client starts from.
func DefaultConfig() Config {
	return Config{
		Timeout:               30 * time.Second,
		DialTimeout:           5 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0,
		MaxRedirects:          10,
		UserAgent:             "purser/0",
	}
}
