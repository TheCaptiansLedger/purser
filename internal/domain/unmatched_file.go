package domain

import "time"

// UnmatchedFileStatus is kernel-owned pipeline state, closed and validated
// like MonitorMode/ItemStatus in enums.go — scoped to this one entity
// rather than shared across entities, so it's defined here instead, the
// same way Gender is defined in person.go rather than enums.go. See
// docs/adr/0024-pipeline-core.md.
type UnmatchedFileStatus string

// The complete set of valid UnmatchedFileStatus values.
const (
	UnmatchedFileStatusPending   UnmatchedFileStatus = "pending"
	UnmatchedFileStatusMatched   UnmatchedFileStatus = "matched"
	UnmatchedFileStatusDismissed UnmatchedFileStatus = "dismissed"
)

// UnmatchedFile is a file the common scan pipeline discovered and hashed
// but has not yet matched to a LibraryEntry/Item — the pre-identification
// state a MediaFile's required ItemID can't represent. See
// docs/adr/0024-pipeline-core.md.
type UnmatchedFile struct {
	ID   string `validate:"required"`
	Path string `validate:"required"`
	Size int64

	OSHash string
	MD5    string
	SHA1   string
	SHA512 string

	DiscoveredAt time.Time
	Status       UnmatchedFileStatus `validate:"required,oneof=pending matched dismissed"`
}

// Validate checks UnmatchedFile's invariants: ID and Path are required,
// and Status must be one of the known values.
func (u *UnmatchedFile) Validate() error {
	return validateStruct(u)
}
