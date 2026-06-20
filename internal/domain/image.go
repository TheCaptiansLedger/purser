package domain

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
