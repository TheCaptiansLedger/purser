package identifier

import (
	"context"
	"log/slog"
	"purser/internal/domain"
	"sort"
)

const durationToleranceMS = 2000

// releaseMBID extracts the MusicBrainz ID from a release's external IDs,
// falling back to the internal ID if none is found.
func releaseMBID(r domain.MusicRelease) string {
	for _, eid := range r.ExternalIDs {
		if eid.Source == domain.SourceMusicBrainz {
			return eid.Value
		}
	}
	return r.ID
}

// FilterReleasesByTrackCount removes releases whose TrackCount does not match
// tags.TotalTracks. Returns all releases unmodified when tags.TotalTracks is 0
// (unknown) or when filtering would eliminate all candidates.
func FilterReleasesByTrackCount(ctx context.Context, root string, tags domain.MusicTagSummary, releases []domain.MusicRelease) []domain.MusicRelease {
	if tags.TotalTracks == 0 || len(releases) == 0 {
		return releases
	}

	var filtered []domain.MusicRelease
	for _, r := range releases {
		if r.TrackCount == tags.TotalTracks {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return releases
	}

	slog.InfoContext(ctx, "release signal", "root", root, "signal", "trackCount",
		"score", float64(len(filtered))/float64(len(releases)),
		"matched", len(filtered), "total", len(releases))
	return filtered
}

// ScoreReleaseYearSignal returns 1.0 when the release year exactly matches
// tags.Year, 0.5 when they differ by exactly one year, and 0.0 otherwise or
// when either year is unknown.
func ScoreReleaseYearSignal(ctx context.Context, release domain.MusicRelease, tags domain.MusicTagSummary) float64 {
	mbid := releaseMBID(release)

	if tags.Year == 0 || release.Date.IsZero() {
		slog.InfoContext(ctx, "release signal", "release_mbid", mbid, "signal", "year",
			"score", 0.0, "matched", 0, "total", 0)
		return 0.0
	}

	releaseYear := release.Date.Year()
	diff := releaseYear - tags.Year
	if diff < 0 {
		diff = -diff
	}

	var score float64
	switch diff {
	case 0:
		score = 1.0
	case 1:
		score = 0.5
	}

	slog.InfoContext(ctx, "release signal", "release_mbid", mbid, "signal", "year",
		"score", score, "matched", 0, "total", 0)
	return score
}

// ScoreReleaseDurationSignal returns the fraction of tracks whose file duration
// (from tags.TrackDurations) is within ±2000ms of the corresponding MBZ track
// duration in mbzDurationsMS. Compares min(len(tags.TrackDurations), len(mbzDurationsMS))
// tracks; returns 0.0 when either list is empty.
func ScoreReleaseDurationSignal(ctx context.Context, releaseMBID string, mbzDurationsMS []int, tags domain.MusicTagSummary) float64 {
	n := len(tags.TrackDurations)
	if len(mbzDurationsMS) < n {
		n = len(mbzDurationsMS)
	}
	if n == 0 {
		slog.InfoContext(ctx, "release signal", "release_mbid", releaseMBID, "signal", "duration",
			"score", 0.0, "matched", 0, "total", 0)
		return 0.0
	}

	var matched int
	for i := 0; i < n; i++ {
		fileMS := tags.TrackDurations[i].Milliseconds()
		mbzMS := int64(mbzDurationsMS[i])
		diff := fileMS - mbzMS
		if diff < 0 {
			diff = -diff
		}
		ok := diff <= durationToleranceMS
		slog.DebugContext(ctx, "track duration",
			"track", i+1, "file_ms", fileMS, "mbz_ms", mbzMS, "ok", ok)
		if ok {
			matched++
		}
	}

	score := float64(matched) / float64(n)
	slog.InfoContext(ctx, "release signal", "release_mbid", releaseMBID, "signal", "duration",
		"score", score, "matched", matched, "total", n)
	return score
}

// ScoreReleaseAcoustIDSignal returns the fraction of file recordings (by MBZ
// Recording MBID resolved from AcoustID lookup) that appear in the release's
// expected recording MBID set. Returns 0.0 when either list is empty or all
// fileRecordingMBIDs are empty.
func ScoreReleaseAcoustIDSignal(ctx context.Context, releaseMBID string, releaseRecordingMBIDs []string, fileRecordingMBIDs []string) float64 {
	if len(releaseRecordingMBIDs) == 0 || len(fileRecordingMBIDs) == 0 {
		slog.InfoContext(ctx, "release signal", "release_mbid", releaseMBID, "signal", "acoustid",
			"score", 0.0, "matched", 0, "total", 0)
		return 0.0
	}

	releaseSet := make(map[string]struct{}, len(releaseRecordingMBIDs))
	for _, mbid := range releaseRecordingMBIDs {
		if mbid != "" {
			releaseSet[mbid] = struct{}{}
		}
	}

	var matched, total int
	for _, mbid := range fileRecordingMBIDs {
		if mbid == "" {
			continue
		}
		total++
		if _, ok := releaseSet[mbid]; ok {
			matched++
		}
	}

	if total == 0 {
		slog.InfoContext(ctx, "release signal", "release_mbid", releaseMBID, "signal", "acoustid",
			"score", 0.0, "matched", 0, "total", 0)
		return 0.0
	}

	score := float64(matched) / float64(total)
	slog.InfoContext(ctx, "release signal", "release_mbid", releaseMBID, "signal", "acoustid",
		"score", score, "matched", matched, "total", total)
	return score
}

// RankReleases sorts releases by their score (looked up by MBZ MBID in scores)
// descending. On a tie, the release with IsDefault==true ranks first. Returns the
// releases in ranked order and logs the top result.
func RankReleases(ctx context.Context, releases []domain.MusicRelease, scores map[string]float64) []domain.MusicRelease {
	type entry struct {
		release domain.MusicRelease
		score   float64
		mbid    string
	}

	entries := make([]entry, len(releases))
	for i, r := range releases {
		mbid := releaseMBID(r)
		entries[i] = entry{release: r, score: scores[mbid], mbid: mbid}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		return entries[i].release.IsDefault && !entries[j].release.IsDefault
	})

	result := make([]domain.MusicRelease, len(entries))
	for i, e := range entries {
		result[i] = e.release
	}

	topMBID := ""
	topScore := 0.0
	if len(entries) > 0 {
		topMBID = entries[0].mbid
		topScore = entries[0].score
	}
	slog.InfoContext(ctx, "release ranked",
		"count", len(result), "top_mbid", topMBID, "top_score", topScore)
	return result
}
