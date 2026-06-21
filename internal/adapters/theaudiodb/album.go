package theaudiodb

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
)

type albumResponse struct {
	Album []albumRecord `json:"album"`
}

type albumRecord struct {
	StrMusicBrainzID string `json:"strMusicBrainzID"`
	StrAlbumThumb    string `json:"strAlbumThumb"`
}

// FetchGroupContent fetches the cover art for a music album identified by its
// MusicBrainz release-group MBID, using the album-mb.php endpoint.
// Returns one ExternalItem with an ImageTypePoster image from strAlbumThumb,
// or an empty slice when the album has no thumbnail.
// Only music is supported; other content types return ErrNotSupported.
func (a *Adapter) FetchGroupContent(ctx context.Context, ct domain.ContentType, groupExternalID string, _, _ int) ([]*domain.ExternalItem, int, error) {
	switch ct {
	case domain.ContentTypeMusic:
		return a.fetchAlbumCover(ctx, groupExternalID)
	default:
		return nil, 0, ports.ErrNotSupported
	}
}

func (a *Adapter) fetchAlbumCover(ctx context.Context, rgMBID string) ([]*domain.ExternalItem, int, error) {
	var resp albumResponse
	if err := a.get(ctx, fmt.Sprintf("album-mb.php?i=%s", url.QueryEscape(rgMBID)), &resp); err != nil {
		return nil, 0, err
	}
	if len(resp.Album) == 0 || resp.Album[0].StrAlbumThumb == "" {
		return []*domain.ExternalItem{}, 0, nil
	}
	r := resp.Album[0]
	return []*domain.ExternalItem{{
		Source:      domain.SourceTheAudioDB,
		ExternalIDs: map[string]string{"mbid": r.StrMusicBrainzID},
		Images:      []domain.ExternalImage{{Type: domain.ImageTypePoster, URL: r.StrAlbumThumb}},
	}}, 1, nil
}
