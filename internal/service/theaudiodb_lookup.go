package service

import (
	"context"
	"purser/internal/ports"
)

// TheAudioDBLookup exposes internal/adapters/theaudiodb's read-only
// artist/album lookup capability directly to a caller — see
// proto/purser/music/v1/theaudiodb.proto's own doc comment for why this
// exists. Depends on ports.TheAudioDBClient directly, the same deliberate,
// narrow exception to "services depend on ports describing a capability,
// not a specific provider" that MusicBrainzSearch already relies on — see
// that port's own doc comment.
type TheAudioDBLookup struct {
	tadb ports.TheAudioDBClient
}

// NewTheAudioDBLookup constructs a TheAudioDBLookup backed by tadb.
func NewTheAudioDBLookup(tadb ports.TheAudioDBClient) *TheAudioDBLookup {
	return &TheAudioDBLookup{tadb: tadb}
}

// LookupArtist fetches one artist by MusicBrainz artist MBID — a thin
// passthrough to ports.TheAudioDBClient.LookupArtist.
func (s *TheAudioDBLookup) LookupArtist(ctx context.Context, mbid string) (*ports.TADBArtist, error) {
	return s.tadb.LookupArtist(ctx, mbid)
}

// LookupAlbum fetches one album by MusicBrainz release-group MBID — a thin
// passthrough to ports.TheAudioDBClient.LookupAlbum.
func (s *TheAudioDBLookup) LookupAlbum(ctx context.Context, releaseGroupMBID string) (*ports.TADBAlbum, error) {
	return s.tadb.LookupAlbum(ctx, releaseGroupMBID)
}
