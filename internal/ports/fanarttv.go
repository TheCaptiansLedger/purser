package ports

import "context"

// FanartTVClient is the port for the single fanart.tv adapter
// (docs/adr/0027-provider-independence.md). Like TheAudioDBClient, this is
// a deliberate exception to "ports describe capability, never a specific
// provider" (docs/adr/0001-hexagonal-architecture.md): fanart.tv gets its
// own port, never shared with TheAudioDBClient — both are image sources
// covering overlapping ground, and per ADR-0027 the server never picks a
// winner between them. Its methods return fanart.tv's own DTO shapes
// below, mirroring webservice.fanart.tv's v3 schema (live-verified
// directly against the real API with a real key during this adapter's
// implementation), not domain types — mapping a result into a domain
// Image is the caller's job (a future Music enrichment consumer), not this
// port's.
//
// LookupArtist returns ErrNotFound when fanart.tv's response carries no
// artist data at all — confirmed live: an unknown MBID answers HTTP 200
// with an empty {} body, not an HTTP error status, the only provider port
// in this package where "not found" isn't signaled by status code. An
// invalid API key is a real, non-ErrNotFound error (confirmed live:
// fanart.tv answers HTTP 401 with {"error":"invalid API key"}).
type FanartTVClient interface {
	// LookupArtist fetches one artist's images plus every one of their
	// release groups' album art, via music/{mbid} — a single call answers
	// both (confirmed live: the response's "albums" map, keyed by
	// release-group MBID, covered all 122 of a well-established artist's
	// release groups in one response).
	LookupArtist(ctx context.Context, mbid string) (*FanartArtist, error)
}

// FanartImage is one image entry in an artist-level image array
// (ArtistThumb, ArtistBackground, ...) or an album's AlbumCover array.
// ID/Likes are quoted JSON strings in the real response, not JSON numbers
// — mapped as string here to decode exactly what the API sends. Lang is
// only ever populated on artist-level arrays (confirmed live, always ""
// for The Beatles' entries, but the field exists on every artist-level
// image); AlbumCover entries never carry it at all.
type FanartImage struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Likes string `json:"likes"`
	Lang  string `json:"lang,omitempty"`
}

// FanartCDArt is one CD-art image entry in an album's CDArt array — like
// FanartImage plus Disc/Size, confirmed live only on some albums (an album
// with no CD-art recorded has no "cdart" key at all, distinct from an
// empty array).
type FanartCDArt struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Likes string `json:"likes"`
	Disc  string `json:"disc"`
	Size  string `json:"size"`
}

// FanartAlbumImages is one release group's art, keyed by release-group
// MBID in FanartArtist.Albums.
type FanartAlbumImages struct {
	AlbumCover []FanartImage `json:"albumcover"`
	CDArt      []FanartCDArt `json:"cdart,omitempty"`
}

// FanartArtist is fanart.tv's music/{mbid} response. Artist4KBackground is
// a field docs/technical/music-data_model.md's original research pass
// didn't document at all — discovered live during this adapter's
// implementation, not a v1/pre-reset carryover.
type FanartArtist struct {
	Name               string                       `json:"name"`
	MBID               string                       `json:"mbid_id"`
	ArtistThumb        []FanartImage                `json:"artistthumb"`
	ArtistBackground   []FanartImage                `json:"artistbackground"`
	Artist4KBackground []FanartImage                `json:"artist4kbackground"`
	HDMusicLogo        []FanartImage                `json:"hdmusiclogo"`
	MusicLogo          []FanartImage                `json:"musiclogo"`
	MusicBanner        []FanartImage                `json:"musicbanner"`
	Albums             map[string]FanartAlbumImages `json:"albums"`
}
