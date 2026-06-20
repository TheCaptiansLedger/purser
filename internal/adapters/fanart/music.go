package fanart

import (
	"context"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
)

// ── fanart.tv response types ──────────────────────────────────────────────────

type fanartImage struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Likes string `json:"likes"`
}

type fanartArtistResponse struct {
	Name             string        `json:"name"`
	MBID             string        `json:"mbid_id"`
	ArtistThumb      []fanartImage `json:"artistthumb"`
	ArtistBackground []fanartImage `json:"artistbackground"`
	HDMusicLogo      []fanartImage `json:"hdmusiclogo"`
	MusicBanner      []fanartImage `json:"musicbanner"`
	CDArt            []fanartImage `json:"cdart"`
}

type fanartAlbum struct {
	AlbumCover []fanartImage `json:"albumcover"`
	CDArt      []fanartImage `json:"cdart"`
}

type fanartAlbumsResponse struct {
	Name   string                 `json:"name"`
	MBID   string                 `json:"mbid_id"`
	Albums map[string]fanartAlbum `json:"albums"`
}

// ── Routing ───────────────────────────────────────────────────────────────────

// FindByExternalID fetches artist-level images for the given content type.
// Only music is implemented; other types return ErrNotSupported until their
// issues are filed.
func (a *Adapter) FindByExternalID(ctx context.Context, ct domain.ContentType, id string) (*domain.ExternalItem, error) {
	switch ct {
	case domain.ContentTypeMusic:
		return a.findMusicByID(ctx, id)
	default:
		return nil, ports.ErrNotSupported
	}
}

// FetchEntryContent fetches sub-entity images (albums for music) for the given
// content type. Only music is implemented; other types return ErrNotSupported.
func (a *Adapter) FetchEntryContent(ctx context.Context, ct domain.ContentType, externalID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	switch ct {
	case domain.ContentTypeMusic:
		items, total, err := a.fetchMusicAlbums(ctx, externalID, page, perPage)
		return nil, items, total, err
	default:
		return nil, nil, 0, ports.ErrNotSupported
	}
}

// FetchGroupContent returns ErrNotSupported — fanart.tv has no per-track images.
func (a *Adapter) FetchGroupContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

// ── Music ─────────────────────────────────────────────────────────────────────

func (a *Adapter) findMusicByID(ctx context.Context, mbid string) (*domain.ExternalItem, error) {
	var resp fanartArtistResponse
	if err := a.get(ctx, fmt.Sprintf("music/%s", mbid), &resp); err != nil {
		return nil, err
	}
	return &domain.ExternalItem{
		Source:     domain.SourceFanart,
		ExternalID: resp.MBID,
		Images:     collectArtistImages(&resp),
	}, nil
}

// fetchMusicAlbums calls /music/albums/{mbid} and returns one ExternalItem per
// release group that has at least one albumcover image. Map keys are sorted for
// deterministic local pagination (fanart.tv returns all albums in one response).
func (a *Adapter) fetchMusicAlbums(ctx context.Context, artistMBID string, page, perPage int) ([]*domain.ExternalItem, int, error) {
	var resp fanartAlbumsResponse
	if err := a.get(ctx, fmt.Sprintf("music/albums/%s", artistMBID), &resp); err != nil {
		return nil, 0, err
	}

	rgMBIDs := make([]string, 0, len(resp.Albums))
	for rgMBID := range resp.Albums {
		rgMBIDs = append(rgMBIDs, rgMBID)
	}
	sort.Strings(rgMBIDs)

	allItems := make([]*domain.ExternalItem, 0, len(rgMBIDs))
	for _, rgMBID := range rgMBIDs {
		album := resp.Albums[rgMBID]
		var images []domain.ExternalImage
		for _, img := range album.AlbumCover {
			if img.URL != "" {
				images = append(images, domain.ExternalImage{Type: domain.ImageTypePoster, URL: img.URL})
			}
		}
		if len(images) == 0 {
			continue
		}
		allItems = append(allItems, &domain.ExternalItem{
			Source:      domain.SourceFanart,
			ExternalIDs: map[string]string{"mbid": rgMBID},
			Images:      images,
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

// ── Image helpers ─────────────────────────────────────────────────────────────

// collectArtistImages maps fanart.tv artist image keys to domain.ImageType values.
// hdmusiclogo and cdart are skipped — they have no corresponding slot in the
// music library_entry image slot table.
func collectArtistImages(resp *fanartArtistResponse) []domain.ExternalImage {
	var images []domain.ExternalImage
	for _, img := range resp.ArtistThumb {
		if img.URL != "" {
			images = append(images, domain.ExternalImage{Type: domain.ImageTypeThumbnail, URL: img.URL})
		}
	}
	for _, img := range resp.ArtistBackground {
		if img.URL != "" {
			images = append(images, domain.ExternalImage{Type: domain.ImageTypeBackground, URL: img.URL})
		}
	}
	for _, img := range resp.MusicBanner {
		if img.URL != "" {
			images = append(images, domain.ExternalImage{Type: domain.ImageTypeBanner, URL: img.URL})
		}
	}
	return images
}
