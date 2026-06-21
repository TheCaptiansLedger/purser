package lastfm

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"purser/internal/domain"
)

// ── Last.fm response types ────────────────────────────────────────────────────

type lfmImage struct {
	Text string `json:"#text"`
	Size string `json:"size"`
}

type lfmArtist struct {
	Name  string     `json:"name"`
	MBID  string     `json:"mbid"`
	Image []lfmImage `json:"image"`
}

type lfmArtistSearchResponse struct {
	Results struct {
		ArtistMatches struct {
			Artist []lfmArtist `json:"artist"`
		} `json:"artistmatches"`
	} `json:"results"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// SearchStudios queries Last.fm for artists by name.
func (a *Adapter) SearchStudios(ctx context.Context, query string, limit int) ([]*domain.ExternalStudio, error) {
	params := url.Values{}
	params.Set("method", "artist.search")
	params.Set("artist", query)
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	slog.Debug("lastfm: SearchStudios", "query", query)

	var resp lfmArtistSearchResponse
	if err := a.get(ctx, params, &resp); err != nil {
		return nil, err
	}

	artists := resp.Results.ArtistMatches.Artist
	out := make([]*domain.ExternalStudio, len(artists))
	for i, ar := range artists {
		out[i] = &domain.ExternalStudio{
			Source:     domain.SourceLastFM,
			ExternalID: ar.MBID,
			Name:       ar.Name,
			ImageURL:   bestImage(ar.Image),
		}
	}
	return out, nil
}
