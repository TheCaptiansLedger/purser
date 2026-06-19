package mbz

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
	"strconv"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzReleaseGroupResponse struct {
	ReleaseGroups []mbzReleaseGroup `json:"release-groups"`
	Count         int               `json:"release-group-count"`
}

type mbzReleaseGroup struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	Releases []mbzRelease `json:"releases"`
}

type mbzRelease struct {
	Date string `json:"date"` // "YYYY", "YYYY-MM", or "YYYY-MM-DD"
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// FetchEntryContent fetches an artist's release groups (albums, EPs, singles, etc.)
// as paginated ExternalGroups. Items is always nil — tracks are fetched later via
// FetchGroupContent once that is implemented.
func (a *Adapter) FetchEntryContent(ctx context.Context, artistMBID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	params := url.Values{}
	params.Set("artist", artistMBID)
	params.Set("inc", "releases")
	params.Set("fmt", "json")
	params.Set("limit", strconv.Itoa(perPage))
	params.Set("offset", strconv.Itoa((page-1)*perPage))
	u := fmt.Sprintf("%srelease-group?%s", a.baseURL, params.Encode())

	var resp mbzReleaseGroupResponse
	if err := a.get(ctx, u, &resp); err != nil {
		return nil, nil, 0, err
	}

	groups := make([]*domain.ExternalGroup, len(resp.ReleaseGroups))
	for i := range resp.ReleaseGroups {
		groups[i] = toExternalGroup(&resp.ReleaseGroups[i])
	}
	return groups, nil, resp.Count, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func toExternalGroup(rg *mbzReleaseGroup) *domain.ExternalGroup {
	return &domain.ExternalGroup{
		Source:     domain.SourceMusicBrainz,
		ExternalID: rg.ID,
		Title:      rg.Title,
		Year:       earliestYear(rg.Releases),
		Number:     0,
	}
}

func earliestYear(releases []mbzRelease) int {
	year := 0
	for _, r := range releases {
		if y := parseYear(r.Date); y > 0 && (year == 0 || y < year) {
			year = y
		}
	}
	return year
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
