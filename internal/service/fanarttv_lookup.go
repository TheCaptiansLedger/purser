package service

import (
	"context"
	"purser/internal/ports"
)

// FanartTVLookup exposes internal/adapters/fanarttv's read-only artist
// lookup capability directly to a caller — see
// proto/purser/music/v1/fanarttv.proto's own doc comment for why this
// exists. Depends on ports.FanartTVClient directly, the same deliberate,
// narrow exception to "services depend on ports describing a capability,
// not a specific provider" that MusicBrainzSearch/TheAudioDBLookup already
// rely on.
type FanartTVLookup struct {
	fanart ports.FanartTVClient
}

// NewFanartTVLookup constructs a FanartTVLookup backed by fanart.
func NewFanartTVLookup(fanart ports.FanartTVClient) *FanartTVLookup {
	return &FanartTVLookup{fanart: fanart}
}

// LookupArtist fetches one artist's images plus every one of their release
// groups' album art, by MusicBrainz artist MBID — a thin passthrough to
// ports.FanartTVClient.LookupArtist.
func (s *FanartTVLookup) LookupArtist(ctx context.Context, mbid string) (*ports.FanartArtist, error) {
	return s.fanart.LookupArtist(ctx, mbid)
}
