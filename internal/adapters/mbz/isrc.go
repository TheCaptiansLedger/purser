package mbz

import (
	"context"
	"fmt"
	"log/slog"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

// mbzISRCResponse is returned by GET /ws/2/isrc/<ISRC>?inc=releases+release-groups.
type mbzISRCResponse struct {
	ISRC       string             `json:"isrc"`
	Recordings []mbzISRCRecording `json:"recordings"`
}

// mbzISRCRecording is a single recording in the ISRC response.
type mbzISRCRecording struct {
	ID       string           `json:"id"`
	Releases []mbzISRCRelease `json:"releases"`
}

// mbzISRCRelease is a release as embedded in the ISRC recording context.
type mbzISRCRelease struct {
	ID           string          `json:"id"`
	ReleaseGroup mbzReleaseGroup `json:"release-group"`
}

// ── ISRCLookupSource ──────────────────────────────────────────────────────────

// LookupISRC resolves a recording ISRC to a Release Group MBID by inspecting the
// releases embedded in the first matching recording. Returns ("", nil) if the
// ISRC is valid but no release-group can be determined.
func (a *Adapter) LookupISRC(ctx context.Context, isrc string) (string, error) {
	u := fmt.Sprintf("%sisrc/%s?inc=releases+release-groups&fmt=json", a.baseURL, isrc)

	var resp mbzISRCResponse
	if err := a.get(ctx, u, &resp); err != nil {
		return "", fmt.Errorf("mbz isrc lookup: %w", err)
	}

	releaseCount := 0
	for _, rec := range resp.Recordings {
		releaseCount += len(rec.Releases)
	}

	rgMBID := ""
	for _, rec := range resp.Recordings {
		for _, rel := range rec.Releases {
			if rel.ReleaseGroup.ID != "" {
				rgMBID = rel.ReleaseGroup.ID
				break
			}
		}
		if rgMBID != "" {
			break
		}
	}

	slog.InfoContext(ctx, "mbz isrc lookup",
		"isrc", isrc, "rg_mbid", rgMBID, "release_count", releaseCount)

	return rgMBID, nil
}
