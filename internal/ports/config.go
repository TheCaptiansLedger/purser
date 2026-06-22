package ports

import "context"

// ConfigService provides a merged, precedence-aware view of runtime configuration.
// Operator values (env vars, YAML file) take priority over DB-stored values, which
// take priority over built-in defaults. Bootstrap keys (server.*, database.*, log.*,
// media.path) are never stored in the DB; they are only set by the operator.
type ConfigService interface {
	// Get returns the effective value for a runtime config key.
	// Precedence: env/file override → DB → built-in default.
	Get(ctx context.Context, key string) (string, error)

	// Set writes a runtime config value to the database.
	// Returns errs.ErrLocked if the key is currently managed by the operator.
	Set(ctx context.Context, key, value string) error

	// IsLocked reports whether the key is managed by the operator
	// (set via env var or YAML file) and therefore not writable via the UI.
	IsLocked(key string) bool

	// LockedKeys returns a map of every operator-managed key to true.
	// Keys absent from the map are not locked. Used to populate the locked
	// field in the GET /api/v1/config response.
	LockedKeys() map[string]bool
}
