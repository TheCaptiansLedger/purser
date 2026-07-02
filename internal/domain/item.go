package domain

import "time"

// DefaultItemDate is the sentinel date used when no release date is known.
// Stored as a real date so ordering is consistent across storage backends.
var DefaultItemDate = time.Date(1970, time.April, 10, 0, 0, 0, 0, time.UTC)

// Item is the leaf content unit: Episode, Scene, Track, JAV Title, or Movie.
//
// For LibraryEntries with Kind=KindMovie, one Item is auto-created when the entry
// is added and shares the entry's metadata — the movie and its item are the same
// logical thing.
type Item struct {
	ID             string
	ContentType    ContentType
	LibraryEntryID string
	GroupID        string // empty for flat content types (adult, jav, movie)
	Title          string
	Overview       string
	Date           time.Time
	Sequence       string // "S01E05", "3", "SSIS-001"; empty for movies
	RuntimeSeconds int
	Monitored      bool
	Status         ItemStatus
	CoverPath      string
	SortKey        string
	People         []ItemPerson
	Tags           []Tag
	ExternalIDs    []ExternalID
	MediaFile      *MediaFile // nil if not on disk
	Metadata       map[string]any
	LockedFields   []string
	AddedAt        time.Time
	UpdatedAt      time.Time
}

// ApplyDefaults ensures required fields have non-zero values and computes the SortKey
// before persistence. Must be called before every save.
func (i *Item) ApplyDefaults() {
	if i.Date.IsZero() {
		i.Date = DefaultItemDate
	}
	i.SortKey = ItemSortKey(i.Date, i.Sequence, i.Title)
}

// HasFile reports whether this item has a media file on disk.
func (i *Item) HasFile() bool {
	return i.MediaFile != nil
}

// IsWanted reports whether this item should be grabbed when a release is found.
// This checks only the item-level flags; hierarchy and person monitoring are
// evaluated by the app service, which has access to the full ancestor chain.
func (i *Item) IsWanted() bool {
	return i.Monitored && i.Status == StatusWanted
}
