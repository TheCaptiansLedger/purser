package ports

import "context"

// TheAudioDBClient is the port for the single TheAudioDB adapter
// (docs/adr/0027-provider-independence.md). Like MusicBrainzClient,
// StashDBClient, and ThePornDBClient, this is a deliberate exception to
// "ports describe capability, never a specific provider"
// (docs/adr/0001-hexagonal-architecture.md): TheAudioDB gets its own port,
// never shared with FanartTVClient — both are image/enrichment sources
// covering overlapping ground, and per ADR-0027 the server never picks a
// winner between them. Its methods return TheAudioDB's own DTO shapes
// below, mirroring theaudiodb.com's REST schema (live-verified directly
// against the real API with a real key during this adapter's
// implementation — see docs/technical/music-data_model.md's earlier
// research pass for the original field inventory), not domain types —
// mapping a result into a domain LibraryEntry/Group is the caller's job (a
// future Music enrichment consumer), not this port's.
//
// Every Lookup* method returns ErrNotFound when TheAudioDB's response
// field is JSON null for an unknown MBID — confirmed live: HTTP 200 with
// body {"artists":null} / {"album":null}. TheAudioDB never answers an
// unknown MBID with an HTTP error status, unlike every other REST-shaped
// provider port in this package (ThePornDB, and — for a genuinely
// malformed request — fanart.tv).
type TheAudioDBClient interface {
	// LookupArtist fetches one artist by MusicBrainz artist MBID, via
	// artist-mb.php.
	LookupArtist(ctx context.Context, mbid string) (*TADBArtist, error)

	// LookupAlbum fetches one album by MusicBrainz release-group MBID, via
	// album-mb.php.
	LookupAlbum(ctx context.Context, releaseGroupMBID string) (*TADBAlbum, error)
}

// TADBArtist is TheAudioDB's artist DTO (artist-mb.php), confirmed live
// against the real API. Every numeric-looking field (Members, Followers,
// Popularity, the year fields, ...) is a quoted JSON string in the real
// response, not a JSON number — mapped as string here to decode exactly
// what the API sends rather than coercing.
//
// Locale-suffixed biography variants (strBiographyDE, strBiographyFR, and
// eleven more — confirmed live, 13 locales total) are deliberately not
// mapped, same "map what's actually useful, not every field observed"
// tradeoff TPDBPerformer already makes for SitePerformers — Biography
// below is strBiography, the default/English text.
type TADBArtist struct {
	ID            string `json:"idArtist"`
	Name          string `json:"strArtist"`
	AlternateName string `json:"strArtistAlternate"`
	Label         string `json:"strLabel"`
	FormedYear    string `json:"intFormedYear"`
	BornYear      string `json:"intBornYear"`
	DiedYear      string `json:"intDiedYear"`
	Disbanded     string `json:"strDisbanded"`
	Style         string `json:"strStyle"`
	Genre         string `json:"strGenre"`
	Mood          string `json:"strMood"`
	Gender        string `json:"strGender"`
	Country       string `json:"strCountry"`
	CountryCode   string `json:"strCountryCode"`
	ISNICode      string `json:"strISNIcode"`
	Members       string `json:"intMembers"`
	Followers     string `json:"intFollowers"`
	Popularity    string `json:"intPopularity"`
	Charted       string `json:"intCharted"`
	Website       string `json:"strWebsite"`
	Facebook      string `json:"strFacebook"`
	Twitter       string `json:"strTwitter"`
	Biography     string `json:"strBiography"`
	MusicBrainzID string `json:"strMusicBrainzID"`
	Locked        string `json:"strLocked"`

	// Image fields — confirmed live, all seven populated for a
	// well-established artist (The Beatles); Fanart2-4 are additional
	// fanart slots beyond the primary Fanart image.
	Thumb     string `json:"strArtistThumb"`
	Logo      string `json:"strArtistLogo"`
	Cutout    string `json:"strArtistCutout"`
	Clearart  string `json:"strArtistClearart"`
	WideThumb string `json:"strArtistWideThumb"`
	Fanart    string `json:"strArtistFanart"`
	Fanart2   string `json:"strArtistFanart2"`
	Fanart3   string `json:"strArtistFanart3"`
	Fanart4   string `json:"strArtistFanart4"`
	Banner    string `json:"strArtistBanner"`
}

// TADBAlbum is TheAudioDB's album DTO (album-mb.php), confirmed live.
// Cross-reference IDs to other catalogs (Discogs, Wikidata, AllMusic, ...)
// are deliberately not mapped — not images, not useful to this codebase
// today, same omission tradeoff TADBArtist's biography locales make.
type TADBAlbum struct {
	ID                  string `json:"idAlbum"`
	ArtistID            string `json:"idArtist"`
	Title               string `json:"strAlbum"`
	ArtistName          string `json:"strArtist"`
	YearReleased        string `json:"intYearReleased"`
	Style               string `json:"strStyle"`
	Genre               string `json:"strGenre"`
	Label               string `json:"strLabel"`
	ReleaseFormat       string `json:"strReleaseFormat"`
	Description         string `json:"strDescription"`
	Mood                string `json:"strMood"`
	MusicBrainzID       string `json:"strMusicBrainzID"`
	MusicBrainzArtistID string `json:"strMusicBrainzArtistID"`
	Locked              string `json:"strLocked"`

	// Image fields — confirmed live; ThumbHQ/Back/Spine/ThreeDFlat/
	// ThreeDFace are null on releases without those assets recorded (seen
	// live on "Please Please Me"), not every field is populated for every
	// album.
	Thumb       string `json:"strAlbumThumb"`
	ThumbHQ     string `json:"strAlbumThumbHQ"`
	Back        string `json:"strAlbumBack"`
	CDArt       string `json:"strAlbumCDart"`
	Spine       string `json:"strAlbumSpine"`
	ThreeDCase  string `json:"strAlbum3DCase"`
	ThreeDFlat  string `json:"strAlbum3DFlat"`
	ThreeDFace  string `json:"strAlbum3DFace"`
	ThreeDThumb string `json:"strAlbum3DThumb"`
}
