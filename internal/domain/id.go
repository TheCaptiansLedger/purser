package domain

import "github.com/google/uuid"

// NewID returns a new server-generated identifier for a kernel entity.
// UUIDv7 embeds a millisecond timestamp, so generated IDs sort
// chronologically by creation time — this keeps Badger/SQL primary-key
// insert locality good and makes ID order a reliable proxy for insertion
// order, unlike pure-random UUIDv4. See
// docs/adr/0020-server-generated-kernel-entity-ids.md.
func NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Only fails if the system's entropy source is unusable — not a
		// condition any caller of NewID could meaningfully recover from.
		panic("domain: generating UUIDv7: " + err.Error())
	}
	return id.String()
}
