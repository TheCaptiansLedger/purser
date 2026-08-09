package domain

// Setting is one DB-stored runtime configuration override — see
// docs/adr/0028-layered-settings.md. It carries no timestamps and no
// generated identity: Key is itself the identity, a natural key rather
// than a domain.NewID() UUIDv7 (docs/adr/0020-server-generated-kernel-entity-ids.md
// governs kernel entities; Setting isn't one, the same reasoning
// docs/adr/0019-tag-identity-and-get-or-create.md applies to Tag's
// identity, simpler here since there's no composite tuple to reserve).
//
// Key is the same dotted namespace docs/adr/0010-configuration.md already
// uses for mapstructure/Viper (e.g. "pipeline.confidence_threshold"), just
// dot- instead of underscore-joined and without the PURSER_ prefix.
//
// Value is always a JSON-encoded scalar, slice, or map — never a bare
// string — so it round-trips through internal/config's DB-overlay merge
// pass (viper.Set after a json.Unmarshal into any) without losing shape
// for non-scalar fields such as a module's roots []string.
type Setting struct {
	Key   string `validate:"required"`
	Value string
}

// Validate checks Setting's one invariant: Key must be present. Value has
// no shape constraint here — a JSON-encoded empty value (e.g. "" or
// "null") is a caller concern, not a domain one; malformed JSON is caught
// by whoever decodes Value, not by Validate.
func (s *Setting) Validate() error {
	return validateStruct(s)
}
