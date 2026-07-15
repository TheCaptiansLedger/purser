// Package music holds the Music module's own domain types — the fourth
// tier (specific pressing/edition) that sits between a kernel Group
// (Release Group) and its Items (tracks), which no other module needs. See
// docs/adr/0021-music-domain-model.md.
package music

import "time"

// ReleaseStatus is Music's own closed, validated acquisition state for a
// Release — distinct from the shared domain.ItemStatus pipeline, since a
// Release can be "known but nothing on disk" (stub) independent of any one
// track's own status.
type ReleaseStatus string

// The complete set of valid ReleaseStatus values.
const (
	ReleaseStatusStub     ReleaseStatus = "stub"
	ReleaseStatusPartial  ReleaseStatus = "partial"
	ReleaseStatusImported ReleaseStatus = "imported"
)

// Release is a specific pressing/edition of a Release Group — MusicBrainz's
// distinction between the logical album (kernel Group) and one particular
// release of it (this type). LibraryEntryID is denormalized from the
// Group's own LibraryEntryID specifically so an artist-scoped "all releases
// by this artist" query doesn't need to join through Group — see
// docs/adr/0021-music-domain-model.md.
//
// MBID and Barcode are plain, indexed fields rather than shared
// domain.ExternalID rows — a deliberate exception documented in
// ADR-0021, not an oversight.
type Release struct {
	ID             string `validate:"required"`
	GroupID        string `validate:"required"`
	LibraryEntryID string `validate:"required"`
	Title          string `validate:"required"`

	Country       string
	Date          *time.Time
	Label         string
	CatalogNumber string
	Barcode       string
	Format        string

	MediumCount int
	TrackCount  int

	IsDefault bool
	Monitored bool
	Status    ReleaseStatus `validate:"required,oneof=stub partial imported"`
	MBID      string

	AddedAt   *time.Time
	UpdatedAt *time.Time
}

// Validate checks Release's invariants: ID, GroupID, LibraryEntryID, and
// Title are required, and Status must be one of the known acquisition
// states.
func (r *Release) Validate() error {
	return validateStruct(r)
}
