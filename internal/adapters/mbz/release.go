package mbz

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzReleaseList struct {
	Releases []mbzReleaseRef `json:"releases"`
}

type mbzReleaseRef struct {
	ID string `json:"id"`
}

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

// FetchGroupContent fetches tracks for a release-group MBID. MusicBrainz
// separates release-groups (the abstract album) from releases (a specific
// pressing), so a two-step lookup is required: first resolve the
// release-group to its canonical release, then fetch track recordings.
// Tracks are flattened across discs; page/perPage slicing is applied locally
// because MusicBrainz does not paginate at the track level.
func (a *Adapter) FetchGroupContent(ctx context.Context, releaseGroupMBID string, page, perPage int) ([]*domain.ExternalItem, int, error) {
	releaseMBID, err := a.resolveToReleaseMBID(ctx, releaseGroupMBID)
	if err != nil {
		return nil, 0, err
	}

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

func (a *Adapter) resolveToReleaseMBID(ctx context.Context, releaseGroupMBID string) (string, error) {
	params := url.Values{}
	params.Set("release-group", releaseGroupMBID)
	params.Set("limit", "1")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%srelease?%s", a.baseURL, params.Encode())

	var list mbzReleaseList
	if err := a.get(ctx, u, &list); err != nil {
		return "", fmt.Errorf("resolving release-group %s: %w", releaseGroupMBID, err)
	}
	if len(list.Releases) == 0 {
		return "", fmt.Errorf("musicbrainz: no releases found for release-group %s", releaseGroupMBID)
	}
	return list.Releases[0].ID, nil
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
