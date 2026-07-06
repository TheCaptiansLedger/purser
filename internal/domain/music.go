package domain

import "time"

// ReleaseStatus tracks how much of a MusicRelease is present in the library.
type ReleaseStatus string

// Release status constants tracking how much of a pressing is present in the library.
const (
	ReleaseStatusStub     ReleaseStatus = "stub"
	ReleaseStatusPartial  ReleaseStatus = "partial"
	ReleaseStatusImported ReleaseStatus = "imported"
)

// MusicRelease is a specific pressing or edition of a Release Group.
type MusicRelease struct {
	ID             string
	GroupID        string
	LibraryEntryID string
	Title          string
	Country        string
	Date           time.Time
	Label          string
	CatalogNumber  string
	Barcode        string
	Format         string
	MediumCount    int
	TrackCount     int
	IsDefault      bool
	Monitored      bool
	Status         ReleaseStatus
	ExternalIDs    []ExternalID
	CoverPath      string
	AddedAt        time.Time
	UpdatedAt      time.Time
}

// ApplyDefaults sets timestamps and a default status on a new MusicRelease.
func (r *MusicRelease) ApplyDefaults() {
	now := time.Now().UTC()
	if r.AddedAt.IsZero() {
		r.AddedAt = now
	}
	if r.UpdatedAt.IsZero() {
		r.UpdatedAt = now
	}
	if r.Status == "" {
		r.Status = ReleaseStatusStub
	}
}
