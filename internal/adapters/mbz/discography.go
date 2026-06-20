package mbz

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"purser/internal/domain"
	"strconv"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzReleaseGroupResponse struct {
	ReleaseGroups []mbzReleaseGroup `json:"release-groups"`
	Count         int               `json:"release-group-count"`
}

// mbzReleaseGroup is the shape returned by the release-group browse endpoint.
// first-release-date is always present in browse responses; inc=releases is NOT
// a valid parameter on that endpoint and will return HTTP 400.
type mbzReleaseGroup struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	FirstReleaseDate string   `json:"first-release-date"`
	PrimaryType      string   `json:"primary-type"`
	SecondaryTypes   []string `json:"secondary-types"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// FetchEntryContent fetches an artist's release groups (albums, EPs, singles, etc.)
// as paginated ExternalGroups. Items is always nil — tracks are fetched via
// FetchGroupContent for each release group.
//
// MBZ browse endpoint: GET /ws/2/release-group?artist=<MBID>&limit=N&offset=N&fmt=json
// first-release-date is included in every release-group object by default.
// inc=releases is NOT supported on this endpoint and returns HTTP 400.
func (a *Adapter) FetchEntryContent(ctx context.Context, _ domain.ContentType, artistMBID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	params := url.Values{}
	params.Set("artist", artistMBID)
	params.Set("fmt", "json")
	params.Set("limit", strconv.Itoa(perPage))
	params.Set("offset", strconv.Itoa((page-1)*perPage))
	u := fmt.Sprintf("%srelease-group?%s", a.baseURL, params.Encode())

	slog.Debug("mbz: FetchEntryContent", "artist", artistMBID, "page", page, "url", u)

	var resp mbzReleaseGroupResponse
	if err := a.get(ctx, u, &resp); err != nil {
		slog.Warn("mbz: FetchEntryContent failed", "artist", artistMBID, "error", err)
		return nil, nil, 0, err
	}

	slog.Debug("mbz: FetchEntryContent result", "artist", artistMBID, "total", resp.Count, "page_count", len(resp.ReleaseGroups))

	groups := make([]*domain.ExternalGroup, len(resp.ReleaseGroups))
	for i := range resp.ReleaseGroups {
		groups[i] = toExternalGroup(&resp.ReleaseGroups[i])
	}
	return groups, nil, resp.Count, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func toExternalGroup(rg *mbzReleaseGroup) *domain.ExternalGroup {
	return &domain.ExternalGroup{
		Source:         domain.SourceMusicBrainz,
		ExternalID:     rg.ID,
		Title:          rg.Title,
		Year:           parseYear(rg.FirstReleaseDate),
		PrimaryType:    rg.PrimaryType,
		SecondaryTypes: rg.SecondaryTypes,
	}
}

func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}
