package domain

// ImageSelection records which Image is the one currently in use for one
// owner+slot — (OwnerType, OwnerID, ImageType) is its whole identity, one
// row per slot, no ID of its own. Image rows are never overwritten or
// superseded in place (a changed image is always a new row, see
// docs/technical/image-caching-and-serving.md's immutability rule); this
// is the pointer that says which of the, possibly several, Image rows for
// a slot is the active one. Attaching a new image sets this; picking an
// older one from the gallery sets this; nothing ever infers "the current
// image" by guessing at row order.
type ImageSelection struct {
	OwnerType string    `validate:"required"`
	OwnerID   string    `validate:"required"`
	ImageType ImageType `validate:"required"`
	ImageID   string    `validate:"required"`
}

// Validate checks ImageSelection's invariants: every field is required.
func (s *ImageSelection) Validate() error {
	return validateStruct(s)
}
