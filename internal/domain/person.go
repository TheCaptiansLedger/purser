package domain

import "time"

// PersonRole describes how a person is credited on a particular item.
type PersonRole string

// Person role constants for all supported credit types.
const (
	RolePerformer PersonRole = "performer"
	RoleActress   PersonRole = "actress"
	RoleDirector  PersonRole = "director"
	RoleActor     PersonRole = "actor"
	RoleArtist    PersonRole = "artist"
	RoleProducer  PersonRole = "producer"
	RoleAuthor    PersonRole = "author"
)

// Person is a performer, actor, artist, or actress who appears in or creates content.
// People are independently monitorable: Person.Monitored=true causes Purser to grab
// all content featuring that person, regardless of which studio produced it.
type Person struct {
	ID           string
	Name         string
	SortName     string
	Overview     string
	Monitored    bool
	MonitorMode  MonitorMode
	ImagePath    string
	Aliases      []string
	ExternalIDs  []ExternalID
	Metadata     map[string]any
	LockedFields []string
	AddedAt      time.Time
}

// ItemPerson links a Person to a specific Item with their credited role.
type ItemPerson struct {
	PersonID string
	Person   *Person // nil unless explicitly loaded
	Role     PersonRole
}

// EntryPerson links a Person to a library entry with their role and optional tenure dates.
// Used for associations that belong to the entity at rest regardless of specific items:
// band members (artist), contracted performers (studio/network), regular cast (series),
// cast and crew (movie), and authors (book).
type EntryPerson struct {
	PersonID  string
	Person    *Person // nil unless explicitly loaded
	Role      string
	StartDate time.Time
	EndDate   time.Time
}
