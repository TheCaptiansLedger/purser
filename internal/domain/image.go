package domain

import "time"

// ImageType identifies the visual slot an image occupies for a given entity.
type ImageType string

// Image type constants for all supported visual slots.
const (
	ImageTypePoster     ImageType = "poster"     // album cover, movie poster, book cover
	ImageTypeThumbnail  ImageType = "thumbnail"  // scene, episode, track
	ImageTypeBanner     ImageType = "banner"     // wide UI header strip
	ImageTypeHero       ImageType = "hero"       // person hero shot, artist/show landing
	ImageTypeBackground ImageType = "background" // fanart/backdrop
)

type imageKey struct {
	entityType  string
	contentType string
}

var imageSlots = map[imageKey][]ImageType{
	{"library_entry", "music"}: {ImageTypeHero, ImageTypeBanner, ImageTypeThumbnail},
	{"library_entry", "tv"}:    {ImageTypePoster, ImageTypeBanner, ImageTypeHero, ImageTypeBackground},
	{"library_entry", "adult"}: {ImageTypePoster, ImageTypeBanner},
	{"library_entry", "movie"}: {ImageTypePoster, ImageTypeBanner, ImageTypeBackground},
	{"library_entry", "jav"}:   {ImageTypePoster, ImageTypeBanner},
	{"library_entry", "book"}:  {ImageTypePoster, ImageTypeBanner},
	{"group", "music"}:         {ImageTypePoster, ImageTypeBanner},
	{"group", "tv"}:            {ImageTypePoster, ImageTypeBanner},
	{"group", "adult"}:         {ImageTypePoster, ImageTypeBanner},
	{"item", "music"}:          {ImageTypeThumbnail},
	{"item", "tv"}:             {ImageTypeThumbnail},
	{"item", "adult"}:          {ImageTypeThumbnail, ImageTypeBanner},
	{"item", "movie"}:          {ImageTypePoster, ImageTypeThumbnail},
	{"item", "jav"}:            {ImageTypeThumbnail, ImageTypeBanner},
}

// StoredImage is a persisted image record in the images table.
type StoredImage struct {
	ID         string
	EntityType string
	EntityID   string
	ImageType  ImageType
	URL        string
	Source     string
	Width      int
	Height     int
	AddedAt    time.Time
}

// ApplicableImageTypes returns the ordered image slots for a (contentType, entityType) pair.
// Person entities apply across all content types; all other combinations must match exactly.
// Returns an empty slice for unrecognized combinations.
func ApplicableImageTypes(contentType, entityType string) []ImageType {
	if entityType == "person" {
		return []ImageType{ImageTypeHero, ImageTypeBanner, ImageTypeThumbnail}
	}
	if slots, ok := imageSlots[imageKey{entityType, contentType}]; ok {
		return slots
	}
	return []ImageType{}
}
