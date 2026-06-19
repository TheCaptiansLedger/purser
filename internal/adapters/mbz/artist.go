package mbz

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzArtistDetail struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Disambiguation string        `json:"disambiguation"`
	Relations      []mbzRelation `json:"relations"`
}

type mbzRelation struct {
	Type string `json:"type"`
	URL  struct {
		Resource string `json:"resource"`
	} `json:"url"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// FindByExternalID fetches a single artist by MBID.
// Returns ports.ErrNotFound when the MBID does not exist in MusicBrainz.
func (a *Adapter) FindByExternalID(ctx context.Context, id string) (*domain.ExternalItem, error) {
	params := url.Values{}
	params.Set("inc", "url-rels")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%sartist/%s?%s", a.baseURL, id, params.Encode())

	var artist mbzArtistDetail
	if err := a.get(ctx, u, &artist); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	return &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  artist.ID,
		ContentType: domain.ContentTypeMusic,
		Title:       artist.Name,
		Overview:    artist.Disambiguation,
	}, nil
}
