package mbz

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzReleaseDetail struct {
	Media []mbzMedium `json:"media"`
}

type mbzMedium struct {
	TrackCount int        `json:"track-count"`
	Tracks     []mbzTrack `json:"tracks"`
}

type mbzTrack struct {
	Title     string `json:"title"`
	Recording struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Length int    `json:"length"` // milliseconds
	} `json:"recording"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// FetchGroupContent fetches all tracks on a release, flattening across discs,
// then applies page/perPage slicing. MusicBrainz does not paginate at the
// track level so the full release is fetched and sliced locally.
func (a *Adapter) FetchGroupContent(ctx context.Context, releaseMBID string, page, perPage int) ([]*domain.ExternalItem, int, error) {
	params := url.Values{}
	params.Set("inc", "recordings")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%srelease/%s?%s", a.baseURL, releaseMBID, params.Encode())

	var release mbzReleaseDetail
	if err := a.get(ctx, u, &release); err != nil {
		return nil, 0, err
	}

	total := 0
	var all []*domain.ExternalItem
	for _, m := range release.Media {
		total += m.TrackCount
		for i := range m.Tracks {
			all = append(all, trackToExternalItem(&m.Tracks[i]))
		}
	}

	offset := (page - 1) * perPage
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + perPage
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func trackToExternalItem(t *mbzTrack) *domain.ExternalItem {
	title := t.Title
	if title == "" {
		title = t.Recording.Title
	}
	return &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  t.Recording.ID,
		ContentType: domain.ContentTypeMusic,
		Title:       title,
		RuntimeSecs: t.Recording.Length / 1000,
	}
}
