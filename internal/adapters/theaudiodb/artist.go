package theaudiodb

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
)

type artistResponse struct {
	Artists []artistRecord `json:"artists"`
}

type artistRecord struct {
	StrArtist        string `json:"strArtist"`
	StrMusicBrainzID string `json:"strMusicBrainzID"`
	StrBiographyEN   string `json:"strBiographyEN"`
	StrArtistThumb   string `json:"strArtistThumb"`
	StrWebsite       string `json:"strWebsite"`
	StrGenre         string `json:"strGenre"`
}

// SearchStudios searches for music artists by name.
func (a *Adapter) SearchStudios(ctx context.Context, query string, limit int) ([]*domain.ExternalStudio, error) {
	var resp artistResponse
	if err := a.get(ctx, "search.php?s="+url.QueryEscape(query), &resp); err != nil {
		return nil, err
	}
	out := make([]*domain.ExternalStudio, 0, len(resp.Artists))
	for _, r := range resp.Artists {
		if limit > 0 && len(out) >= limit {
			break
		}
		if r.StrMusicBrainzID == "" {
			continue
		}
		out = append(out, &domain.ExternalStudio{
			Source:     domain.SourceTheAudioDB,
			ExternalID: r.StrMusicBrainzID,
			Name:       r.StrArtist,
			Overview:   r.StrBiographyEN,
			ImageURL:   r.StrArtistThumb,
			WebsiteURL: r.StrWebsite,
		})
	}
	return out, nil
}

// FindByExternalID fetches an artist by MusicBrainz ID.
// Only music is supported; other content types return ErrNotSupported.
func (a *Adapter) FindByExternalID(ctx context.Context, ct domain.ContentType, id string) (*domain.ExternalItem, error) {
	switch ct {
	case domain.ContentTypeMusic:
		return a.findArtistByMBID(ctx, id)
	default:
		return nil, ports.ErrNotSupported
	}
}

func (a *Adapter) findArtistByMBID(ctx context.Context, mbid string) (*domain.ExternalItem, error) {
	var resp artistResponse
	if err := a.get(ctx, fmt.Sprintf("artist-mb.php?i=%s", url.QueryEscape(mbid)), &resp); err != nil {
		return nil, err
	}
	if len(resp.Artists) == 0 {
		return nil, ports.ErrNotFound
	}
	r := resp.Artists[0]
	return &domain.ExternalItem{
		Source:      domain.SourceTheAudioDB,
		ExternalID:  r.StrMusicBrainzID,
		ContentType: domain.ContentTypeMusic,
		Title:       r.StrArtist,
		Overview:    r.StrBiographyEN,
	}, nil
}
