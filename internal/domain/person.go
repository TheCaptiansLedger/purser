package domain

import "time"

// Gender is a closed, inclusive enum. Values are taken directly from
// StashDB's live GenderEnum (confirmed by GraphQL introspection during
// design research, not guessed), plus Unknown as our own fallback.
//
// Adapters own translation into this enum — a provider's own gender
// representation (TMDB's numeric codes, ThePornDB's free-text "Unknown")
// is mapped to one of these values at ingest time, in the adapter, never
// in the domain. A value with no confident mapping becomes GenderUnknown,
// not a guess.
type Gender string

// The complete set of valid Gender values.
const (
	GenderMale              Gender = "MALE"
	GenderFemale            Gender = "FEMALE"
	GenderTransgenderMale   Gender = "TRANSGENDER_MALE"
	GenderTransgenderFemale Gender = "TRANSGENDER_FEMALE"
	GenderIntersex          Gender = "INTERSEX"
	GenderNonBinary         Gender = "NON_BINARY"
	GenderUnknown           Gender = "UNKNOWN"
)

// String satisfies fmt.Stringer.
func (g Gender) String() string {
	return string(g)
}

// Valid reports whether g is one of the known Gender values.
func (g Gender) Valid() bool {
	switch g {
	case GenderMale, GenderFemale, GenderTransgenderMale, GenderTransgenderFemale,
		GenderIntersex, GenderNonBinary, GenderUnknown:
		return true
	default:
		return false
	}
}

// Person is an individual human — not an act, band, or studio. A band's
// members are Person records; the band itself is a LibraryEntry. A solo
// artist gets both: a LibraryEntry for the act and a separate Person for
// the human, linked by an EntryPerson, the same as a band — never one
// record doing double duty. See docs/technical/shared-domain-model.md for
// the reasoning.
//
// Person carries only what's true regardless of role. Role-specific rich
// metadata (a performer's measurements, an author's bibliography) lives in
// a module-owned Profile type that references Person by ID — never here.
type Person struct {
	ID       string `validate:"required"`
	Name     string `validate:"required"`
	SortName string
	Aliases  []string

	Gender   Gender `validate:"required,oneof=MALE FEMALE TRANSGENDER_MALE TRANSGENDER_FEMALE INTERSEX NON_BINARY UNKNOWN"`
	Pronouns string

	BirthDate *time.Time
	DeathDate *time.Time

	Nationality string
	Overview    string

	Monitored   bool
	MonitorMode MonitorMode `validate:"required,oneof=all future none latest"`

	AddedAt   time.Time
	UpdatedAt time.Time
}

// Validate checks Person's invariants: a stable ID and Name are required,
// Gender must be one of the known values (never a raw provider string),
// and MonitorMode must be one of the known monitoring modes.
func (p *Person) Validate() error {
	return validateStruct(p)
}
