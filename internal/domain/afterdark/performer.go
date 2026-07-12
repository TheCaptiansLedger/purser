package afterdark

// BodyMark is a located, described physical mark — a tattoo or piercing.
// Both fields are free text (StashDB/ThePornDB return unstructured
// location/description pairs, per docs/technical/afterdark-data_model.md).
type BodyMark struct {
	Location    string
	Description string
}

// PerformerProfile is AfterDark's role-specific extension of a kernel
// domain.Person — physical/biographical attributes that only make sense
// in an AfterDark context. It references its Person by PersonID rather
// than embedding domain.Person, per
// docs/technical/shared-domain-model.md Part 3: storage stays dumb, and a
// service composes a PerformerView from this plus domain.Person only at
// the read/API boundary, never persisted as a combined row.
//
// Deliberately thin beyond PersonID — StashDB's structured fields and
// ThePornDB's free-text equivalents both collapse into the same sparse
// shape here; provider-specific parsing is adapter knowledge, not domain
// knowledge, per ADR 0001.
type PerformerProfile struct {
	PersonID string `validate:"required"`

	CupSize    string
	BandSize   string
	BreastType string

	Tattoos   []BodyMark
	Piercings []BodyMark

	CareerStartYear int
	CareerEndYear   int
}

// Validate checks PerformerProfile's invariants: PersonID is required.
func (p *PerformerProfile) Validate() error {
	return validateStruct(p)
}
