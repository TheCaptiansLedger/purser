// Package datastore declares the generic, entity-agnostic persistence
// abstraction behind Purser's Badger and SQL backends. See
// docs/adr/0012-datastore-persistence.md.
//
// Datastore is deliberately not a purser/internal/ports interface: ports
// are what internal/service depends on, per
// docs/adr/0001-hexagonal-architecture.md. Datastore is one layer further
// down, consumed only by the entity-specific ports.XRepository
// implementations in purser/internal/adapters/store — the service layer
// never imports this package and never knows it exists.
package datastore

import "context"

// Document is the opaque unit Datastore stores and retrieves. An
// internal/adapters/store/<entity> repository is the only thing that
// assigns meaning to Collection/ID/Index — Datastore itself never inspects
// Data and never learns what an Index key or value means.
type Document struct {
	// Collection groups documents of the same entity type, e.g. "person",
	// "image", "entry_person".
	Collection string

	// ID is opaque to Datastore. For single-key entities it's the
	// entity's own ID; for composite-key entities (EntryPerson,
	// ItemPerson, ExternalID) the owning repository joins the key parts
	// into one string before calling Datastore — see
	// internal/adapters/store.CompositeRepository's keySeparator.
	ID string

	// Data is the JSON-encoded domain entity.
	Data []byte

	// Index holds flat, opaque key/value pairs the owning repository
	// wants filterable via List's filter argument — e.g.
	// {"owner_type": "person", "owner_id": "p1"} for Image. Datastore
	// persists and indexes these without knowing what they mean.
	Index map[string]string
}

// Datastore is the narrow persistence port every entity repository in
// internal/adapters/store is built on. Both implementations
// (internal/adapters/datastore/badger, internal/adapters/datastore/sql)
// satisfy it identically from the caller's point of view — see
// docs/adr/0012-datastore-persistence.md's LSP note.
type Datastore interface {
	// Create stores doc. Returns purser/internal/ports.ErrConflict if a
	// document with the same Collection+ID already exists.
	Create(ctx context.Context, doc Document) error

	// Get returns the document stored under collection+id. Returns
	// purser/internal/ports.ErrNotFound if none exists.
	Get(ctx context.Context, collection, id string) (Document, error)

	// Update replaces the document stored under doc.Collection+doc.ID.
	// Returns purser/internal/ports.ErrNotFound if none exists.
	Update(ctx context.Context, doc Document) error

	// Delete removes the document stored under collection+id. Returns
	// purser/internal/ports.ErrNotFound if none exists.
	Delete(ctx context.Context, collection, id string) error

	// List returns documents in collection, cursor-paginated by ID. A
	// non-empty filter restricts results to documents whose Index
	// contains every given key/value pair (AND semantics) via an indexed
	// lookup, not a full-collection scan — see
	// docs/adr/0012-datastore-persistence.md. A zero-value pageToken
	// starts from the beginning; a non-empty nextPageToken is returned
	// whenever more results exist, and is empty on the last page.
	List(ctx context.Context, collection string, filter map[string]string, pageSize int, pageToken string) (docs []Document, nextPageToken string, err error)

	// CreateBatch stores every doc in docs as a single transaction — all
	// succeed or none do. Returns purser/internal/ports.ErrConflict if any
	// doc's Collection+ID already exists, rolling back the whole batch. See
	// docs/adr/0016-bulk-operations.md. Only entities with a real bulk-create
	// API endpoint call this — most callers keep using Create.
	CreateBatch(ctx context.Context, docs []Document) error

	// DeleteBatch removes every document stored under collection+id for
	// each id in ids, as a single transaction — all succeed or none do.
	// Returns purser/internal/ports.ErrNotFound if any id doesn't exist,
	// rolling back the whole batch. See docs/adr/0016-bulk-operations.md.
	// Only entities with a real bulk-delete API endpoint call this — most
	// callers keep using Delete.
	DeleteBatch(ctx context.Context, collection string, ids []string) error
}
