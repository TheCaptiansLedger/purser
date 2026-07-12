package domain

// ImageType names what role an image plays (hero, poster, banner...).
// Open string, not a closed set — a module Profile type may need an image
// slot the kernel didn't anticipate, and that must not require editing
// this file. The constants below are known values from Music's v1 image
// handling, generalized to any owner.
type ImageType string

// Known ImageType values, generalized from Music's v1 image-slot handling.
const (
	ImageTypeHero       ImageType = "hero"
	ImageTypePoster     ImageType = "poster"
	ImageTypeBanner     ImageType = "banner"
	ImageTypeBackground ImageType = "background"
	ImageTypeThumb      ImageType = "thumb"
)

// Image is a polymorphic attachment, same pattern as Tag and ExternalID:
// any owner can have images by (OwnerType, OwnerID) without a dedicated
// field predeclared on every struct that might ever have one.
//
// This is the mechanism behind the kernel-image / module-image split:
// OwnerType "person" is the safe, cross-context image shown on the global
// People page. A module's own Profile type (e.g. an AfterDark performer
// profile) attaches its own images under its own OwnerType, and a service
// composing a module-specific view decides which set to show — see
// docs/technical/shared-domain-model.md Part 3. Neither Person nor a
// module's Profile type needs to know the other's images exist.
type Image struct {
	ID        string    `validate:"required"`
	OwnerType string    `validate:"required"`
	OwnerID   string    `validate:"required"`
	ImageType ImageType `validate:"required"`
	URL       string    `validate:"required"`
	Width     int
	Height    int
	Source    string
	Priority  int
}

// Validate checks Image's invariants: ID, OwnerType, OwnerID, ImageType,
// and URL are all required.
func (img *Image) Validate() error {
	return validateStruct(img)
}
