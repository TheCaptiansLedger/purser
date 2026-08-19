package service

import (
	"context"
	"purser/internal/ports"
)

// MusicBrainzSearch exposes internal/adapters/musicbrainz's read-only
// search/browse capability directly to a caller — see
// proto/purser/music/v1/musicbrainz_search.proto's own doc comment for
// why this exists. Depends on ports.MusicBrainzClient directly, the same
// deliberate, narrow exception to "services depend on ports describing a
// capability, not a specific provider" that
// adapters/pipeline/music.Identifier already relies on — see that port's
// own doc comment ("MusicBrainz is a singular identity graph with exactly
// one real implementation, not a swappable capability").
type MusicBrainzSearch struct {
	mb ports.MusicBrainzClient
}

// NewMusicBrainzSearch constructs a MusicBrainzSearch backed by mb.
func NewMusicBrainzSearch(mb ports.MusicBrainzClient) *MusicBrainzSearch {
	return &MusicBrainzSearch{mb: mb}
}

// SearchReleaseGroups free-text searches MusicBrainz release groups by
// artist and album name — a thin passthrough to
// ports.MusicBrainzClient.SearchReleaseGroups. Search/List methods never
// return ports.ErrNotFound for a zero-result query (see that port's own
// doc comment) — an empty slice is a valid, non-error result here too.
func (s *MusicBrainzSearch) SearchReleaseGroups(ctx context.Context, artistName, albumName string) ([]ports.ReleaseGroup, error) {
	return s.mb.SearchReleaseGroups(ctx, artistName, albumName)
}

// ListReleasesForReleaseGroup lists every known pressing/edition of a
// release group by release-group MBID — a thin passthrough to
// ports.MusicBrainzClient.ListReleasesForReleaseGroup.
func (s *MusicBrainzSearch) ListReleasesForReleaseGroup(ctx context.Context, releaseGroupMBID string) ([]ports.Release, error) {
	return s.mb.ListReleasesForReleaseGroup(ctx, releaseGroupMBID)
}

// SearchArtists free-text searches MusicBrainz artists by name — a thin
// passthrough to ports.MusicBrainzClient.SearchArtists. An empty slice is
// a valid, non-error result here too (see that port's own doc comment).
func (s *MusicBrainzSearch) SearchArtists(ctx context.Context, query string) ([]ports.Artist, error) {
	return s.mb.SearchArtists(ctx, query)
}

// ListReleaseGroupsForArtist lists every release group credited to one
// artist by artist MBID — a thin passthrough to
// ports.MusicBrainzClient.ListReleaseGroupsForArtist.
func (s *MusicBrainzSearch) ListReleaseGroupsForArtist(ctx context.Context, artistMBID string) ([]ports.ReleaseGroup, error) {
	return s.mb.ListReleaseGroupsForArtist(ctx, artistMBID)
}

// GetArtist looks up one artist by MBID — a thin passthrough to
// ports.MusicBrainzClient.LookupArtist. Unlike the Search/List methods
// above, an unknown MBID is an error here: LookupArtist returns
// ports.ErrNotFound, which the caller maps to CodeNotFound.
func (s *MusicBrainzSearch) GetArtist(ctx context.Context, mbid string) (*ports.Artist, error) {
	return s.mb.LookupArtist(ctx, mbid)
}

// GetRelease looks up one release by MBID, including its full track
// listing — a thin passthrough to ports.MusicBrainzClient.LookupRelease.
// Like GetArtist, an unknown MBID is an error: LookupRelease returns
// ports.ErrNotFound, which the caller maps to CodeNotFound.
func (s *MusicBrainzSearch) GetRelease(ctx context.Context, mbid string) (*ports.Release, error) {
	return s.mb.LookupRelease(ctx, mbid)
}
