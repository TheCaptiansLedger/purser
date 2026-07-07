package identifier_test

import (
	"context"
	"fmt"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

func mbzRelease(id, title string, year int, trackCount int, isDefault bool) domain.MusicRelease {
	var date time.Time
	if year != 0 {
		date = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return domain.MusicRelease{
		ID:         id,
		Title:      title,
		Date:       date,
		TrackCount: trackCount,
		IsDefault:  isDefault,
		ExternalIDs: []domain.ExternalID{
			{Source: domain.SourceMusicBrainz, Value: id},
		},
	}
}

// ── FilterReleasesByTrackCount ────────────────────────────────────────────────

func TestFilterReleasesByTrackCount_RemovesNonMatching(t *testing.T) {
	tags := domain.MusicTagSummary{TotalTracks: 10}
	releases := []domain.MusicRelease{
		mbzRelease("r1", "Hi Infidelity", 1981, 10, true),
		mbzRelease("r2", "Hi Infidelity (Bonus)", 1981, 12, false),
		mbzRelease("r3", "Hi Infidelity (Promo)", 1982, 8, false),
	}

	got := identifier.FilterReleasesByTrackCount(context.Background(), "/music/test", tags, releases)
	if len(got) != 1 {
		t.Fatalf("expected 1 release after filtering, got %d", len(got))
	}
	if got[0].ID != "r1" {
		t.Errorf("expected r1, got %q", got[0].ID)
	}
}

func TestFilterReleasesByTrackCount_UnknownTrackCount_ReturnsAll(t *testing.T) {
	tags := domain.MusicTagSummary{TotalTracks: 0}
	releases := []domain.MusicRelease{
		mbzRelease("r1", "A", 1981, 10, true),
		mbzRelease("r2", "B", 1981, 12, false),
	}
	got := identifier.FilterReleasesByTrackCount(context.Background(), "/music/test", tags, releases)
	if len(got) != 2 {
		t.Errorf("expected all 2 releases returned when TotalTracks=0, got %d", len(got))
	}
}

func TestFilterReleasesByTrackCount_AllEliminated_ReturnsAll(t *testing.T) {
	tags := domain.MusicTagSummary{TotalTracks: 5}
	releases := []domain.MusicRelease{
		mbzRelease("r1", "A", 1981, 10, true),
		mbzRelease("r2", "B", 1981, 12, false),
	}
	got := identifier.FilterReleasesByTrackCount(context.Background(), "/music/test", tags, releases)
	if len(got) != 2 {
		t.Errorf("expected fallback to all 2 releases when all eliminated, got %d", len(got))
	}
}

// ── ScoreReleaseYearSignal ────────────────────────────────────────────────────

func TestScoreReleaseYearSignal_ExactMatch(t *testing.T) {
	r := mbzRelease("r1", "Hi Infidelity", 1981, 10, true)
	tags := domain.MusicTagSummary{Year: 1981}
	score := identifier.ScoreReleaseYearSignal(context.Background(), r, tags)
	if score != 1.0 {
		t.Errorf("exact year match: got %v, want 1.0", score)
	}
}

func TestScoreReleaseYearSignal_YearUnknown(t *testing.T) {
	r := mbzRelease("r1", "Hi Infidelity", 1981, 10, true)
	tags := domain.MusicTagSummary{Year: 0}
	score := identifier.ScoreReleaseYearSignal(context.Background(), r, tags)
	if score != 0.0 {
		t.Errorf("unknown tag year: got %v, want 0.0", score)
	}
}

func TestScoreReleaseYearSignal_OffByOne(t *testing.T) {
	r := mbzRelease("r1", "Hi Infidelity", 1981, 10, true)
	tags := domain.MusicTagSummary{Year: 1982}
	score := identifier.ScoreReleaseYearSignal(context.Background(), r, tags)
	if score != 0.5 {
		t.Errorf("off-by-one year: got %v, want 0.5", score)
	}
}

func TestScoreReleaseYearSignal_NoDateOnRelease(t *testing.T) {
	r := domain.MusicRelease{ID: "r1", Title: "Unknown Date"}
	tags := domain.MusicTagSummary{Year: 1981}
	score := identifier.ScoreReleaseYearSignal(context.Background(), r, tags)
	if score != 0.0 {
		t.Errorf("zero release date: got %v, want 0.0", score)
	}
}

// ── ScoreReleaseDurationSignal ────────────────────────────────────────────────

func TestScoreReleaseDurationSignal_AllMatch(t *testing.T) {
	mbzMS := []int{213000, 203000, 212000}
	tags := domain.MusicTagSummary{
		TrackDurations: []time.Duration{
			213000 * time.Millisecond,
			203000 * time.Millisecond,
			212000 * time.Millisecond,
		},
	}
	score := identifier.ScoreReleaseDurationSignal(context.Background(), "r1", mbzMS, tags)
	if score != 1.0 {
		t.Errorf("all tracks match: got %v, want 1.0", score)
	}
}

func TestScoreReleaseDurationSignal_OneOff(t *testing.T) {
	mbzMS := []int{213000, 203000, 212000}
	tags := domain.MusicTagSummary{
		TrackDurations: []time.Duration{
			213000 * time.Millisecond,
			203000 * time.Millisecond,
			// 5s off — outside ±2s tolerance
			217000 * time.Millisecond,
		},
	}
	score := identifier.ScoreReleaseDurationSignal(context.Background(), "r1", mbzMS, tags)
	const want = 2.0 / 3.0
	if score < want-0.001 || score > want+0.001 {
		t.Errorf("one track off: got %v, want ~%.4f", score, want)
	}
}

func TestScoreReleaseDurationSignal_EmptyLists_ReturnsZero(t *testing.T) {
	score := identifier.ScoreReleaseDurationSignal(context.Background(), "r1", nil, domain.MusicTagSummary{})
	if score != 0.0 {
		t.Errorf("empty lists: got %v, want 0.0", score)
	}
}

func TestScoreReleaseDurationSignal_WithinTolerance(t *testing.T) {
	// 1999ms diff — just inside ±2000ms
	mbzMS := []int{213000}
	tags := domain.MusicTagSummary{
		TrackDurations: []time.Duration{214999 * time.Millisecond},
	}
	score := identifier.ScoreReleaseDurationSignal(context.Background(), "r1", mbzMS, tags)
	if score != 1.0 {
		t.Errorf("within tolerance: got %v, want 1.0", score)
	}
}

// TestScoreReleaseDurationSignal_Integration_REOSpeedwagon builds 10 synthetic FLAC
// files for REO Speedwagon "Hi Infidelity" with LENGTH tags encoding real track
// durations. Extracts MusicTagSummary from the fingerprinted files, then scores
// against the matching MBZ track list. Score must be ≥0.95 (all 10 tracks match).
func TestScoreReleaseDurationSignal_Integration_REOSpeedwagon(t *testing.T) {
	// Hi Infidelity release 1e639bf3 track durations from MusicBrainz (milliseconds).
	mbzDurationsMS := []int{
		213626, // Don't Let Him Go
		203360, // Keep on Loving You
		212000, // Follow My Heart
		209000, // In Your Letter
		238000, // Take It On the Run
		190000, // Tough Guys
		233000, // Out of Season
		185000, // Shakin' It Loose
		251000, // Someone Tonight
		227000, // I Wish You Were There
	}

	tracks := []struct {
		num        int
		title      string
		isrc       string
		durationMS int
	}{
		{1, "Don't Let Him Go", "USSM10012807", mbzDurationsMS[0]},
		{2, "Keep on Loving You", "USSM10012808", mbzDurationsMS[1]},
		{3, "Follow My Heart", "USSM10012809", mbzDurationsMS[2]},
		{4, "In Your Letter", "USSM10012810", mbzDurationsMS[3]},
		{5, "Take It On the Run", "USSM10012811", mbzDurationsMS[4]},
		{6, "Tough Guys", "USSM10012812", mbzDurationsMS[5]},
		{7, "Out of Season", "USSM10012813", mbzDurationsMS[6]},
		{8, "Shakin' It Loose", "USSM10012814", mbzDurationsMS[7]},
		{9, "Someone Tonight", "USSM10012815", mbzDurationsMS[8]},
		{10, "I Wish You Were There", "USSM10012816", mbzDurationsMS[9]},
	}

	scanFiles := make([]domain.ScannedFile, 0, len(tracks))
	for _, tr := range tracks {
		sf := fingerprintedFile(t, fmt.Sprintf("reo_%02d", tr.num), map[string]string{
			"ALBUMARTIST":          "REO Speedwagon",
			"ALBUM":                "Hi Infidelity",
			"TITLE":                tr.title,
			"TRACKNUMBER":          fmt.Sprintf("%d", tr.num),
			"TRACKTOTAL":           "10",
			"DISCNUMBER":           "1",
			"DISCTOTAL":            "1",
			"DATE":                 "1981",
			"BARCODE":              "0074646161425",
			"ISRC":                 tr.isrc,
			"MUSICBRAINZ ALBUM ID": "1e639bf3-6b4c-4e1a-9d15-c61511804c8f",
			"LENGTH":               fmt.Sprintf("%d", tr.durationMS),
		})
		scanFiles = append(scanFiles, sf)
	}

	group := ports.ScannedFileGroup{RootPath: "/music/REO Speedwagon/Hi Infidelity", Files: scanFiles}
	summary := identifier.ExtractMusicTagSummary(context.Background(), group)
	t.Logf("TrackDurations: %v", summary.TrackDurations)

	score := identifier.ScoreReleaseDurationSignal(
		context.Background(),
		"1e639bf3-6b4c-4e1a-9d15-c61511804c8f",
		mbzDurationsMS,
		summary,
	)
	t.Logf("duration score = %.4f", score)

	if score < 0.95 {
		t.Errorf("duration score = %.4f, want ≥0.95", score)
	}
}

// ── ScoreReleaseAcoustIDSignal ────────────────────────────────────────────────

func TestScoreReleaseAcoustIDSignal_AllMatch(t *testing.T) {
	releaseIDs := []string{"rec-a", "rec-b", "rec-c"}
	fileIDs := []string{"rec-a", "rec-b", "rec-c"}
	score := identifier.ScoreReleaseAcoustIDSignal(context.Background(), "r1", releaseIDs, fileIDs)
	if score != 1.0 {
		t.Errorf("all match: got %v, want 1.0", score)
	}
}

func TestScoreReleaseAcoustIDSignal_PartialMatch(t *testing.T) {
	releaseIDs := []string{"rec-a", "rec-b", "rec-c"}
	fileIDs := []string{"rec-a", "rec-b", "rec-x"}
	score := identifier.ScoreReleaseAcoustIDSignal(context.Background(), "r1", releaseIDs, fileIDs)
	const want = 2.0 / 3.0
	if score < want-0.001 || score > want+0.001 {
		t.Errorf("partial match: got %v, want ~%.4f", score, want)
	}
}

func TestScoreReleaseAcoustIDSignal_EmptyFileIDs_ReturnsZero(t *testing.T) {
	releaseIDs := []string{"rec-a"}
	score := identifier.ScoreReleaseAcoustIDSignal(context.Background(), "r1", releaseIDs, nil)
	if score != 0.0 {
		t.Errorf("empty file IDs: got %v, want 0.0", score)
	}
}

func TestScoreReleaseAcoustIDSignal_AllEmpty_ReturnsZero(t *testing.T) {
	score := identifier.ScoreReleaseAcoustIDSignal(context.Background(), "r1", nil, nil)
	if score != 0.0 {
		t.Errorf("both empty: got %v, want 0.0", score)
	}
}

// ── RankReleases ─────────────────────────────────────────────────────────────

func TestRankReleases_DefaultWinsTie(t *testing.T) {
	releases := []domain.MusicRelease{
		mbzRelease("r1", "Hi Infidelity", 1981, 10, false),
		mbzRelease("r2", "Hi Infidelity (Default)", 1981, 10, true),
	}
	scores := map[string]float64{"r1": 0.80, "r2": 0.80}

	ranked := identifier.RankReleases(context.Background(), releases, scores)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(ranked))
	}
	if ranked[0].ID != "r2" {
		t.Errorf("expected default release (r2) first on tie, got %q", ranked[0].ID)
	}
}

func TestRankReleases_HigherScoreWins(t *testing.T) {
	releases := []domain.MusicRelease{
		mbzRelease("r1", "A", 1981, 10, true),
		mbzRelease("r2", "B", 1981, 10, false),
	}
	scores := map[string]float64{"r1": 0.60, "r2": 0.95}

	ranked := identifier.RankReleases(context.Background(), releases, scores)
	if ranked[0].ID != "r2" {
		t.Errorf("expected higher-scored release (r2) first, got %q", ranked[0].ID)
	}
}

func TestRankReleases_EmptyInput(t *testing.T) {
	ranked := identifier.RankReleases(context.Background(), nil, nil)
	if len(ranked) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(ranked))
	}
}
