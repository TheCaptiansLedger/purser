package domain

// MediaFile is a file on disk backing an Item. Content-hash/fingerprint
// identification (AcoustID, StashDB/ThePornDB PHash) deliberately isn't a
// first-class field here — that's pipeline/acquisition-core territory, not
// a content-metadata concern, and gets its own design pass later. Until
// then, per-module fingerprint values live in Metadata, same as v1 did for
// AcoustID.
type MediaFile struct {
	ID     string `validate:"required"`
	ItemID string `validate:"required"`
	Path   string `validate:"required"`
	Size   int64

	OSHash string
	MD5    string
	SHA1   string

	Quality    string
	Resolution string
	Codec      string
	Container  string

	Metadata map[string]string
}

// Validate checks MediaFile's invariants: ID, ItemID, and Path are
// required.
func (m *MediaFile) Validate() error {
	return validateStruct(m)
}
