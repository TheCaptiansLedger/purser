package domain

import "time"

// EntryPerson links a Person to a LibraryEntry — used when the association
// belongs to the title itself, regardless of which specific file/episode
// is on disk: a band's members, a series' regular cast, a movie's cast and
// crew, a book's author.
//
// Role is an open string, not a fixed enum: every module's actual role
// vocabulary turned out to be different and open-ended (TMDB's job field,
// Hardcover's free-text contributor role, AfterDark/Music's own role
// sets). Each module validates against its own known-roles list in its own
// config/adapter code, per ADR 0001 rule 3 — role vocabulary is adapter
// knowledge, not shared domain knowledge.
//
// CreditedAs and Character are both optional: CreditedAs is a
// credited/stage name that differs from the person's canonical name for
// this credit (StashDB's performers[].as, Music's artist-credit name);
// Character is a fictional character name (TMDB's cast credits). Neither
// Music nor AfterDark's original join needed these — Movies and TV do, for
// basically every cast credit.
type EntryPerson struct {
	LibraryEntryID string `validate:"required"`
	PersonID       string `validate:"required"`
	Role           string `validate:"required"`
	CreditedAs     string
	Character      string
	StartDate      *time.Time
	EndDate        *time.Time
}

// Validate checks EntryPerson's invariants: LibraryEntryID, PersonID, and
// Role are required.
func (e *EntryPerson) Validate() error {
	return validateStruct(e)
}

// ItemPerson links a Person to an Item — used when the association
// belongs to one specific instance, not the title as a whole: a track's
// featured artist, a scene's performers, an episode's guest star. See
// EntryPerson for the Role/CreditedAs/Character notes, which apply
// identically here.
type ItemPerson struct {
	ItemID     string `validate:"required"`
	PersonID   string `validate:"required"`
	Role       string `validate:"required"`
	CreditedAs string
	Character  string
}

// Validate checks ItemPerson's invariants: ItemID, PersonID, and Role are
// required.
func (i *ItemPerson) Validate() error {
	return validateStruct(i)
}
