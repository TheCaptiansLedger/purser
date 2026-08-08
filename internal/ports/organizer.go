package ports

import (
	"context"
	"errors"
	"purser/internal/domain"
)

// Organizer is the capability of moving/renaming an already-created
// MediaFile to its content type's configured naming-template destination.
// Implemented structurally by service.Organizer (the generic,
// content-type-agnostic render/move mechanics) and consumed here so a
// content type's Persister (an adapter) can trigger auto-organize
// immediately after creating a MediaFile without importing internal/service
// directly. See docs/adr/0024-pipeline-core.md,
// docs/technical/pipeline-music-organizer.md.
type Organizer interface {
	// Organize renders mediaFileID's naming template, moves the file to
	// the computed destination, and returns the MediaFile with its updated
	// Path.
	Organize(ctx context.Context, mediaFileID string) (*domain.MediaFile, error)
}

// ErrDestinationExists is returned by an Organizer implementation when the
// computed destination path already has something at it — a template bug,
// or a leftover from an interrupted previous organize. Never silently
// overwritten: on the manual RPC path this surfaces directly to the
// caller; on the automatic path (from a content type's Persister), it's
// logged and the file is simply left where it is, never blocking the
// overall persist operation from succeeding. See
// docs/technical/pipeline-music-organizer.md's "Collision handling"
// section.
var ErrDestinationExists = errors.New("organizer: destination already exists")

// ErrDestinationOutsideRoot is returned when a rendered naming template
// produces a path that escapes its configured OrganizeConfig.Root (e.g. a
// "../" segment from unexpected metadata). An Organizer implementation
// refuses rather than writing outside the tree an operator configured.
var ErrDestinationOutsideRoot = errors.New("organizer: rendered destination escapes configured root")
