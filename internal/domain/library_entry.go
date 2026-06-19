package domain

import "time"

// LibraryEntry is a node in the content hierarchy tree.
//
// Nodes form a tree via ParentID: network → studio, studio → series, etc.
// Setting Monitored=true on any node propagates wanted status down to all
// descendant items according to MonitorMode.
type LibraryEntry struct {
	ID                string
	ContentType       ContentType
	Kind              Kind
	Name              string
	SortName          string
	Overview          string
	ParentID          string        // empty for root nodes
	Parent            *LibraryEntry // nil unless explicitly loaded
	Monitored         bool
	MonitorMode       MonitorMode
	Status            EntryStatus
	QualityProfileID  string
	MetadataProfileID string
	Path              string
	ImagePath         string
	ExternalIDs       []ExternalID
	Tags              []Tag
	People            []EntryPerson
	Metadata          map[string]any
	AddedAt           time.Time
	UpdatedAt         time.Time
}

// IsRoot reports whether this entry has no parent in the hierarchy.
func (e *LibraryEntry) IsRoot() bool {
	return e.ParentID == ""
}

// DisplayName returns SortName when set, falling back to Name.
func (e *LibraryEntry) DisplayName() string {
	if e.SortName != "" {
		return e.SortName
	}
	return e.Name
}
