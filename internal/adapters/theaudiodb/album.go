package theaudiodb

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
)

type discographyResponse struct {
	Album []discographyRecord `json:"album"`
}

type discographyRecord struct {
	StrAlbum         string `json:"strAlbum"`
	StrMusicBrainzID string `json:"strMusicBrainzID"`
	StrAlbumThumb    string `json:"strAlbumThumb"`
}

// FetchEntryContent fetches album art for a music artist by MBID.
// Calls discography-mb.php to get all albums with their release-group MBIDs
// and strAlbumThumb cover URLs, mirroring the Fanart.tv pattern so that
// fetchAlbumCovers in the service can use both sources interchangeably.
// Only music is supported; other content types return ErrNotSupported.
func (a *Adapter) FetchEntryContent(ctx context.Context, ct domain.ContentType, externalID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	switch ct {
	case domain.ContentTypeMusic:
		items, total, err := a.fetchDiscography(ctx, externalID, page, perPage)
		return nil, items, total, err
	default:
		return nil, nil, 0, ports.ErrNotSupported
	}
}

func (a *Adapter) fetchDiscography(ctx context.Context, artistMBID string, page, perPage int) ([]*domain.ExternalItem, int, error) {
	var resp discographyResponse
	if err := a.get(ctx, fmt.Sprintf("discography-mb.php?s=%s", url.QueryEscape(artistMBID)), &resp); err != nil {
		return nil, 0, err
	}

	var allItems []*domain.ExternalItem
	for _, r := range resp.Album {
		if r.StrMusicBrainzID == "" || r.StrAlbumThumb == "" {
			continue
		}
		allItems = append(allItems, &domain.ExternalItem{
			Source:      domain.SourceTheAudioDB,
			ExternalIDs: map[string]string{"mbid": r.StrMusicBrainzID},
			Images:      []domain.ExternalImage{{Type: domain.ImageTypePoster, URL: r.StrAlbumThumb}},
		})
	}

	total := len(allItems)
	start := (page - 1) * perPage
	if start >= total {
		return []*domain.ExternalItem{}, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return allItems[start:end], total, nil
}
