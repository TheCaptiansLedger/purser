package domain

// ContentType identifies the category of media content.
type ContentType string

// Content type constants for all supported media categories.
const (
	ContentTypeMovie ContentType = "movie"
	ContentTypeTV    ContentType = "tv"
	ContentTypeMusic ContentType = "music"
	ContentTypeAdult ContentType = "adult"
	ContentTypeJAV   ContentType = "jav"
	ContentTypeBook  ContentType = "book"
)

// Valid reports whether c is a known content type.
func (c ContentType) Valid() bool {
	switch c {
	case ContentTypeMovie, ContentTypeTV, ContentTypeMusic, ContentTypeAdult, ContentTypeJAV, ContentTypeBook:
		return true
	}
	return false
}

// Kind is the role a LibraryEntry plays in the content hierarchy.
type Kind string

// Kind constants for all roles a LibraryEntry can play in the hierarchy.
const (
	KindNetwork   Kind = "network"   // AfterDark parent brand (network→studio); tag value for TV series (key=network); not a tree parent for TV
	KindStudio    Kind = "studio"    // production company, adult site, JAV studio; tree parent for KindMovie when a studio is known
	KindSeries    Kind = "series"    // TV show, adult site named series
	KindArtist    Kind = "artist"    // music recording artist or band
	KindAuthor    Kind = "author"    // book author; root of book hierarchy
	KindMovie     Kind = "movie"     // collapsed: entry IS the leaf; tree parent is KindStudio when a studio is known
	KindPublisher Kind = "publisher" // tag-only (key=publisher); not a tree node
	KindBook      Kind = "book"      // collapsed: entry IS the leaf book
)

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	switch k {
	case KindNetwork, KindStudio, KindSeries, KindArtist, KindAuthor, KindMovie, KindPublisher, KindBook:
		return true
	}
	return false
}

// RefreshJobName returns the background job name used to refresh entries of this kind.
func (k Kind) RefreshJobName() string {
	if k == KindArtist {
		return "RefreshArtist"
	}
	return "RefreshStudio"
}

// SupportsAlbumFilter reports whether entries of this kind honour the album_filter
// metadata key, which restricts which release types are imported during refresh.
func (k Kind) SupportsAlbumFilter() bool {
	return k == KindArtist
}

// SupportsMemberRelationships reports whether entries of this kind can have
// member relationships (e.g. band members, ensemble performers).
func (k Kind) SupportsMemberRelationships() bool {
	return k == KindArtist
}

// MonitorMode controls how newly discovered children of an entry are handled.
type MonitorMode string

// Monitor mode constants controlling how newly discovered children are handled.
const (
	MonitorAll    MonitorMode = "all"    // backfill everything + grab future
	MonitorFuture MonitorMode = "future" // only items released after entry was added
	MonitorNone   MonitorMode = "none"   // add to library, manual selection only
	MonitorLatest MonitorMode = "latest" // only the single most recently released item; default for studios
)

// EntryStatus represents the production or release state of a library entry.
type EntryStatus string

// Entry status constants for production/release state of a library entry.
const (
	EntryStatusContinuing EntryStatus = "continuing"
	EntryStatusEnded      EntryStatus = "ended"
	EntryStatusActive     EntryStatus = "active"
)
