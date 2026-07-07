package identifier_test

import (
	"context"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"testing"
)

func TestStripAlbumSuffixes(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Hi Infidelity (Remaster)", "Hi Infidelity"},
		{"Hi Infidelity (2024 Remaster)", "Hi Infidelity"},
		{"The Wall (Deluxe Edition)", "The Wall"},
		{"The Wall [Deluxe Edition]", "The Wall"},
		{"Dark Side of the Moon (50th Anniversary Edition)", "Dark Side of the Moon"},
		{"Kind of Blue (Live)", "Kind of Blue"},
		{"Kind of Blue (Live at Newport)", "Kind of Blue"},
		{"Thriller (Special Edition)", "Thriller"},
		{"Purple Rain (Expanded Edition)", "Purple Rain"},
		{"Born to Run (30th Anniversary)", "Born to Run"},
		{"No Jacket Required (Bonus Track Version)", "No Jacket Required"},
		{"Normal Album", "Normal Album"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := identifier.StripAlbumSuffixes(tc.input)
			if got != tc.want {
				t.Errorf("StripAlbumSuffixes(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestScoreBarcodeSignal_Hit(t *testing.T) {
	tags := domain.MusicTagSummary{Barcode: "0074646161425", AlbumTitle: "Hi Infidelity", TotalTracks: 10}
	c := domain.MusicReleaseCandidate{ReleaseGroupTitle: "Hi Infidelity", ReleaseBarcode: "0074646161425", ReleaseTrackCount: 10}
	score := identifier.ScoreBarcodeSignal(context.Background(), "/music/test", tags, c)
	if score != 1.0 {
		t.Errorf("expected 1.0 on barcode match, got %v", score)
	}
}

func TestScoreBarcodeSignal_Miss(t *testing.T) {
	tags := domain.MusicTagSummary{Barcode: "0074646161425", AlbumTitle: "Hi Infidelity", TotalTracks: 10}
	c := domain.MusicReleaseCandidate{ReleaseGroupTitle: "Hi Infidelity", ReleaseBarcode: "9999999999999", ReleaseTrackCount: 10}
	score := identifier.ScoreBarcodeSignal(context.Background(), "/music/test", tags, c)
	if score != 0.0 {
		t.Errorf("expected 0.0 on barcode miss, got %v", score)
	}
}

func TestScoreISRCSignal_AllAgree(t *testing.T) {
	tags := domain.MusicTagSummary{
		AlbumTitle:  "Hi Infidelity",
		ISRCs:       []string{"USSM10012807", "USSM10012808", "USSM10012809"},
		TotalTracks: 3,
	}
	score := identifier.ScoreISRCSignal(context.Background(), "/music/test", tags,
		[]string{"USSM10012807", "USSM10012808", "USSM10012809"})
	if score != 1.0 {
		t.Errorf("expected 1.0 when all ISRCs agree, got %v", score)
	}
}

func TestScoreISRCSignal_PartialAgreement(t *testing.T) {
	tags := domain.MusicTagSummary{ISRCs: []string{"A", "B", "C"}}
	score := identifier.ScoreISRCSignal(context.Background(), "/music/test", tags, []string{"A", "B", "D"})
	const want = 2.0 / 3.0
	if score < want-0.001 || score > want+0.001 {
		t.Errorf("expected ~%.4f on partial ISRC agreement, got %v", want, score)
	}
}

func TestScoreISRCSignal_NoneAgree(t *testing.T) {
	tags := domain.MusicTagSummary{ISRCs: []string{"A", "B"}}
	score := identifier.ScoreISRCSignal(context.Background(), "/music/test", tags, []string{"C", "D"})
	if score != 0.0 {
		t.Errorf("expected 0.0 when no ISRCs agree, got %v", score)
	}
}

func TestScoreFuzzyNameSignal_ExactAfterStrip(t *testing.T) {
	tags := domain.MusicTagSummary{AlbumTitle: "Hi Infidelity (2024 Remaster)"}
	candidates := []domain.MusicReleaseCandidate{{ReleaseGroupTitle: "Hi Infidelity"}}
	score := identifier.ScoreFuzzyNameSignal(context.Background(), "/music/test", tags, candidates)
	if score < 1.0 {
		t.Errorf("expected exact match (1.0) after strip, got %v", score)
	}
}

func TestScoreFuzzyNameSignal_LiveVsStudio(t *testing.T) {
	tags := domain.MusicTagSummary{AlbumTitle: "Hi Infidelity"}
	candidates := []domain.MusicReleaseCandidate{
		{ReleaseGroupTitle: "Live in the Heartland"},
		{ReleaseGroupTitle: "Hi Infidelity"},
	}
	score := identifier.ScoreFuzzyNameSignal(context.Background(), "/music/test", tags, candidates)
	if score < 1.0 {
		t.Errorf("expected studio album as top match (1.0), got %v", score)
	}
}

func TestFilterByTrackCount_EliminatesNonMatching(t *testing.T) {
	tags := domain.MusicTagSummary{TotalTracks: 10}
	candidates := []domain.MusicReleaseCandidate{
		{ReleaseGroupTitle: "Hi Infidelity", ReleaseTrackCount: 10},
		{ReleaseGroupTitle: "Hi Infidelity (Bonus)", ReleaseTrackCount: 12},
		{ReleaseGroupTitle: "Hi Infidelity (Promo)", ReleaseTrackCount: 8},
	}
	got := identifier.FilterByTrackCount(context.Background(), "/music/test", tags, candidates)
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate after filtering, got %d", len(got))
	}
	if got[0].ReleaseTrackCount != 10 {
		t.Errorf("retained candidate has %d tracks, want 10", got[0].ReleaseTrackCount)
	}
}
