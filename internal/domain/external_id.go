package domain

// ExternalIDSource identifies an external metadata database.
type ExternalIDSource string

// External ID source constants for supported metadata databases.
const (
	SourceStashDB     ExternalIDSource = "stashdb"
	SourceTPDB        ExternalIDSource = "tpdb"
	SourceTMDB        ExternalIDSource = "tmdb"
	SourceTVDB        ExternalIDSource = "tvdb"
	SourceMusicBrainz ExternalIDSource = "mbz"
	SourceJavLibrary  ExternalIDSource = "javlibrary"
	SourceR18         ExternalIDSource = "r18"
	SourceFanart      ExternalIDSource = "fanart"
	SourceTheAudioDB  ExternalIDSource = "audiodb"
	SourceDiscogs     ExternalIDSource = "discogs"
	SourceMAL         ExternalIDSource = "mal"
	SourceAniList     ExternalIDSource = "anilist"
)

// ExternalID links a domain entity to a record in an external metadata database.
type ExternalID struct {
	Source ExternalIDSource
	Value  string
}
