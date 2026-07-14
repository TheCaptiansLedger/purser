// Package ports declares the narrow, capability-specific interfaces the
// service layer depends on. See docs/adr/0001-hexagonal-architecture.md and
// docs/adr/0002-solid-design-principles.md.
package ports

import "errors"

// ErrNotFound is returned by any repository method that can't find the
// requested record. Every adapter returns this sentinel, never a
// backend-specific "not found" error, so callers can check for it without
// knowing which adapter is behind the port — see
// docs/adr/0011-api-design.md's error-mapping convention.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a write would violate a uniqueness
// invariant the repository enforces (e.g. creating a record whose ID
// already exists).
var ErrConflict = errors.New("conflict")

// ErrDeletionBlocked is returned by a composing deletion service's Delete
// when the target has structural referrers (a required foreign key, e.g.
// a Group's LibraryEntryID) that can't simply be unlinked, and the caller
// didn't explicitly request cascade=true. See
// docs/adr/0015-deletion-impact-and-composing-services.md.
var ErrDeletionBlocked = errors.New("deletion blocked: dependent records exist")
