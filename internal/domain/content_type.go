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

// SupportsImportEnrichment reports whether entries of this kind should be
// fully enriched on first import: metadata keys, release group and release
// stubs, and member people fetched inline so the entry is immediately browsable.
func (k Kind) SupportsImportEnrichment() bool {
	return k == KindArtist
}

// ContentTypes returns all known content types in a stable order.
func ContentTypes() []ContentType {
	return []ContentType{ContentTypeMovie, ContentTypeTV, ContentTypeMusic, ContentTypeAdult, ContentTypeJAV, ContentTypeBook}
}

// Kinds returns all known entry kinds in a stable order.
func Kinds() []Kind {
	return []Kind{KindNetwork, KindStudio, KindSeries, KindArtist, KindAuthor, KindMovie, KindPublisher, KindBook}
}

// ItemPersonRoles returns valid person roles for items of this content type.
func (c ContentType) ItemPersonRoles() []string {
	switch c {
	case ContentTypeAdult, ContentTypeJAV:
		return []string{"performer", "actress", "actor", "director"}
	case ContentTypeTV:
		return []string{"actor", "actress", "director", "guest_star", "writer"}
	case ContentTypeMovie:
		return []string{"actor", "actress", "director", "producer", "writer"}
	case ContentTypeMusic:
		return []string{"artist", "featured_artist", "producer", "songwriter"}
	case ContentTypeBook:
		return []string{"author", "editor", "illustrator", "narrator"}
	default:
		return []string{"performer"}
	}
}

// EntryPersonRoles returns valid person roles for entries of this kind.
func (k Kind) EntryPersonRoles() []string {
	switch k {
	case KindArtist:
		return []string{"member", "former_member", "vocalist", "guitarist", "bassist", "drummer", "keyboardist", "producer"}
	case KindStudio:
		return []string{"performer", "director", "contracted_performer"}
	case KindNetwork:
		return []string{"affiliated_performer", "director", "producer"}
	case KindSeries:
		return []string{"regular_cast", "recurring_cast", "director", "producer", "writer"}
	case KindMovie:
		return []string{"actor", "actress", "director", "producer", "writer"}
	case KindBook, KindAuthor:
		return []string{"author", "editor", "narrator", "illustrator"}
	case KindPublisher:
		return []string{"author", "editor"}
	default:
		return []string{"member"}
	}
}

// EntryLabel returns the human-readable plural name for library entries of this kind.
func (k Kind) EntryLabel() string {
	switch k {
	case KindArtist:
		return "Artists"
	case KindStudio:
		return "Studios"
	case KindSeries:
		return "Series"
	case KindNetwork:
		return "Networks"
	case KindMovie:
		return "Movies"
	case KindBook:
		return "Books"
	case KindAuthor:
		return "Authors"
	case KindPublisher:
		return "Publishers"
	default:
		return "Library Entries"
	}
}

// GroupLabel returns the human-readable name for groups belonging to this content type.
func (c ContentType) GroupLabel() string {
	switch c {
	case ContentTypeMusic:
		return "Albums"
	case ContentTypeTV:
		return "Seasons"
	case ContentTypeAdult, ContentTypeJAV:
		return "Series"
	default:
		return "Groups"
	}
}

// ItemLabel returns the human-readable name for items belonging to this content type.
func (c ContentType) ItemLabel() string {
	switch c {
	case ContentTypeMusic:
		return "Tracks"
	case ContentTypeTV:
		return "Episodes"
	case ContentTypeAdult, ContentTypeJAV:
		return "Scenes"
	case ContentTypeMovie:
		return "Movies"
	case ContentTypeBook:
		return "Chapters"
	default:
		return "Items"
	}
}

// Label returns the human-readable display name for this content type.
func (c ContentType) Label() string {
	switch c {
	case ContentTypeMovie:
		return "Movies"
	case ContentTypeTV:
		return "TV"
	case ContentTypeMusic:
		return "Music"
	case ContentTypeAdult:
		return "AfterDark"
	case ContentTypeJAV:
		return "JAV"
	case ContentTypeBook:
		return "Books"
	default:
		return string(c)
	}
}

// ModuleKey returns the config module key that enables this content type.
// Both adult and jav map to the "afterdark" module.
func (c ContentType) ModuleKey() string {
	switch c {
	case ContentTypeMovie:
		return "movies"
	case ContentTypeTV:
		return "tv"
	case ContentTypeMusic:
		return "music"
	case ContentTypeAdult, ContentTypeJAV:
		return "afterdark"
	case ContentTypeBook:
		return "books"
	default:
		return string(c)
	}
}

// SupportsFileGrouping reports whether items of this content type can be
// grouped by an intermediate entity (album) in the unmatched-file queue.
func (c ContentType) SupportsFileGrouping() bool {
	return c == ContentTypeMusic
}

// ParentEntryKind returns the Kind string for the root LibraryEntry that
// owns items of this content type. Empty when the content type has no
// fixed parent kind.
func (c ContentType) ParentEntryKind() string {
	switch c {
	case ContentTypeMusic:
		return string(KindArtist)
	case ContentTypeAdult, ContentTypeJAV:
		return string(KindStudio)
	case ContentTypeTV:
		return string(KindSeries)
	case ContentTypeMovie:
		return string(KindMovie)
	case ContentTypeBook:
		return string(KindAuthor)
	default:
		return ""
	}
}

// MediaExtensions returns the file extensions (lowercased, with dot) that are
// considered media files for this content type.
// Example: ContentTypeMusic → [".flac", ".mp3", ".m4a", ".ogg", ".opus", ".wav", ".aiff"]
func (ct ContentType) MediaExtensions() []string {
	switch ct {
	case ContentTypeMovie, ContentTypeTV, ContentTypeAdult, ContentTypeJAV:
		return []string{".mkv", ".mp4", ".avi", ".mov", ".m4v", ".wmv"}
	case ContentTypeMusic:
		return []string{".flac", ".mp3", ".m4a", ".ogg", ".opus", ".wav", ".aiff"}
	case ContentTypeBook:
		return []string{".epub", ".pdf", ".mobi", ".azw3", ".cbz", ".cbr"}
	default:
		return nil
	}
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
