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
	StrArtistFanart  string `json:"strArtistFanart"`
	StrArtistFanart2 string `json:"strArtistFanart2"`
	StrArtistFanart3 string `json:"strArtistFanart3"`
	StrArtistBanner  string `json:"strArtistBanner"`
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
		ImageURL:    r.StrArtistThumb,
		Images:      collectArtistImages(r),
	}, nil
}

// FetchStudioThumb returns the artist thumbnail URL for the given MusicBrainz ID.
// Used to enrich search results that have an MBID but no image.
func (a *Adapter) FetchStudioThumb(ctx context.Context, mbid string) (string, error) {
	item, err := a.findArtistByMBID(ctx, mbid)
	if err != nil {
		return "", err
	}
	return item.ImageURL, nil
}

// FetchPersonImage returns the hero image for the artist identified by MBID.
func (a *Adapter) FetchPersonImage(ctx context.Context, extID string) (*domain.ExternalImage, error) {
	item, err := a.findArtistByMBID(ctx, extID)
	if err != nil {
		return nil, err
	}
	for _, img := range item.Images {
		if img.Type == domain.ImageTypeHero {
			img.Source = string(domain.SourceTheAudioDB)
			return &img, nil
		}
	}
	return nil, ports.ErrNotFound
}

func collectArtistImages(r artistRecord) []domain.ExternalImage {
	var images []domain.ExternalImage
	if r.StrArtistThumb != "" {
		images = append(images, domain.ExternalImage{Type: domain.ImageTypeHero, URL: r.StrArtistThumb})
	}
	for _, u := range []string{r.StrArtistFanart, r.StrArtistFanart2, r.StrArtistFanart3} {
		if u != "" {
			images = append(images, domain.ExternalImage{Type: domain.ImageTypeBackground, URL: u})
		}
	}
	if r.StrArtistBanner != "" {
		images = append(images, domain.ExternalImage{Type: domain.ImageTypeBanner, URL: r.StrArtistBanner})
	}
	return images
}
