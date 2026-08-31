package service

import (
	"context"
	"purser/internal/ports"
)

// WikidataLookup exposes internal/adapters/wikidata's read-only image
// lookup capability directly to a caller — see
// proto/purser/music/v1/wikidata.proto's own doc comment for why this
// exists. Depends on ports.WikidataClient directly, the same deliberate,
// narrow exception to "services depend on ports describing a capability,
// not a specific provider" that FanartTVLookup/TheAudioDBLookup already
// rely on.
type WikidataLookup struct {
	wikidata ports.WikidataClient
}

// NewWikidataLookup constructs a WikidataLookup backed by wikidata.
func NewWikidataLookup(wikidata ports.WikidataClient) *WikidataLookup {
	return &WikidataLookup{wikidata: wikidata}
}

// LookupImage fetches every Commons file resolved from entityURL's P18
// claim — a thin passthrough to ports.WikidataClient.LookupImage.
func (s *WikidataLookup) LookupImage(ctx context.Context, entityURL string) ([]ports.WikidataImage, error) {
	return s.wikidata.LookupImage(ctx, entityURL)
}
