package domain

// ExternalIDSource identifies an external metadata database.
type ExternalIDSource string

const (
	SourceStashDB     ExternalIDSource = "stashdb"
	SourceTPDB        ExternalIDSource = "tpdb"
	SourceTMDB        ExternalIDSource = "tmdb"
	SourceTVDB        ExternalIDSource = "tvdb"
	SourceMusicBrainz ExternalIDSource = "mbz"
	SourceJavLibrary  ExternalIDSource = "javlibrary"
	SourceR18         ExternalIDSource = "r18"
	SourceDiscogs     ExternalIDSource = "discogs"
	SourceMAL         ExternalIDSource = "mal"
	SourceAniList     ExternalIDSource = "anilist"
)

// ExternalID links a domain entity to a record in an external metadata database.
type ExternalID struct {
	Source ExternalIDSource
	Value  string
}
