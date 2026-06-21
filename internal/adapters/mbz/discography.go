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

// mbzReleaseBrowseResponse is returned by GET /ws/2/release?artist=<MBID>.
type mbzReleaseBrowseResponse struct {
	Releases []mbzReleaseBrowse `json:"releases"`
	Count    int                `json:"release-count"`
}

// mbzReleaseBrowse is a single entry in the release browse response.
// Only the embedded release-group is used.
type mbzReleaseBrowse struct {
	ReleaseGroup mbzReleaseGroup `json:"release-group"`
}

// mbzReleaseGroup is the shape of a release-group object as embedded inside a
// release (via inc=release-groups). first-release-date is always present.
type mbzReleaseGroup struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	FirstReleaseDate string   `json:"first-release-date"`
	PrimaryType      string   `json:"primary-type"`
	SecondaryTypes   []string `json:"secondary-types"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// releasePageSize is the number of releases to request per internal page.
// MusicBrainz accepts a maximum of 100.
const releasePageSize = 100

// FetchEntryContent returns unique release-groups for an artist, restricted to
// those that have at least one Official release.
//
// MusicBrainz only accepts status= on the /release endpoint, not /release-group.
// This method browses Official releases with inc=release-groups, deduplicates
// across all internal pages, then applies page/perPage to the unique set.
func (a *Adapter) FetchEntryContent(ctx context.Context, _ domain.ContentType, artistMBID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	all, err := a.collectOfficialReleaseGroups(ctx, artistMBID)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("fetch albums page %d: %w", page, err)
	}

	total := len(all)
	start := (page - 1) * perPage
	if start >= total {
		return nil, nil, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return all[start:end], nil, total, nil
}

// collectOfficialReleaseGroups paginates through all Official releases for
// artistMBID and returns the unique release-groups in first-seen order.
func (a *Adapter) collectOfficialReleaseGroups(ctx context.Context, artistMBID string) ([]*domain.ExternalGroup, error) {
	seen := make(map[string]struct{})
	var groups []*domain.ExternalGroup

	offset := 0
	for {
		params := url.Values{}
		params.Set("artist", artistMBID)
		params.Set("status", "official")
		params.Set("inc", "release-groups")
		params.Set("fmt", "json")
		params.Set("limit", strconv.Itoa(releasePageSize))
		params.Set("offset", strconv.Itoa(offset))
		u := fmt.Sprintf("%srelease?%s", a.baseURL, params.Encode())

		slog.Debug("mbz: collectOfficialReleaseGroups", "artist", artistMBID, "offset", offset)

		var resp mbzReleaseBrowseResponse
		if err := a.get(ctx, u, &resp); err != nil {
			slog.Warn("mbz: FetchEntryContent failed", "artist", artistMBID, "offset", offset, "error", err)
			return nil, err
		}

		for i := range resp.Releases {
			rg := &resp.Releases[i].ReleaseGroup
			if rg.ID == "" {
				continue
			}
			if _, exists := seen[rg.ID]; !exists {
				seen[rg.ID] = struct{}{}
				groups = append(groups, toExternalGroup(rg))
			}
		}

		offset += releasePageSize
		if offset >= resp.Count {
			break
		}
	}

	slog.Debug("mbz: collectOfficialReleaseGroups done", "artist", artistMBID, "unique_release_groups", len(groups))
	return groups, nil
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
