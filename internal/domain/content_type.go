package domain

// ContentType identifies the category of media content.
type ContentType string

const (
	ContentTypeMovie ContentType = "movie"
	ContentTypeTV    ContentType = "tv"
	ContentTypeMusic ContentType = "music"
	ContentTypeAdult ContentType = "adult"
	ContentTypeJAV   ContentType = "jav"
	ContentTypeBook  ContentType = "book"
)

func (c ContentType) Valid() bool {
	switch c {
	case ContentTypeMovie, ContentTypeTV, ContentTypeMusic, ContentTypeAdult, ContentTypeJAV, ContentTypeBook:
		return true
	}
	return false
}

// Kind is the role a LibraryEntry plays in the content hierarchy.
type Kind string

const (
	KindNetwork   Kind = "network"   // HBO, Naughty America parent brand, Columbia Records
	KindStudio    Kind = "studio"    // production company, adult site, JAV studio
	KindSeries    Kind = "series"    // TV show, adult site named series
	KindArtist    Kind = "artist"    // music recording artist or band
	KindMovie     Kind = "movie"     // collapsed: entry IS the leaf
	KindPublisher Kind = "publisher" // book publisher (Penguin, Tor, etc.)
	KindBook      Kind = "book"      // collapsed: entry IS the leaf book
)

func (k Kind) Valid() bool {
	switch k {
	case KindNetwork, KindStudio, KindSeries, KindArtist, KindMovie, KindPublisher, KindBook:
		return true
	}
	return false
}

// MonitorMode controls how newly discovered children of an entry are handled.
type MonitorMode string

const (
	MonitorAll    MonitorMode = "all"    // backfill everything + grab future
	MonitorFuture MonitorMode = "future" // only items released after entry was added
	MonitorNone   MonitorMode = "none"   // add to library, manual selection only
	MonitorLatest MonitorMode = "latest" // only the single most recently released item; default for studios
)

// EntryStatus represents the production or release state of a library entry.
type EntryStatus string

const (
	EntryStatusContinuing EntryStatus = "continuing"
	EntryStatusEnded      EntryStatus = "ended"
	EntryStatusActive     EntryStatus = "active"
)
