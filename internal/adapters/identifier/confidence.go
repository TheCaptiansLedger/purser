package identifier

import (
	"purser/internal/domain"
	"strconv"
	"strings"
)

// computeRecordingConfidence returns the recording-level confidence and the
// individual signal contributions that produced it.
//
// Answers "Is this audio the specific recording titled 'title' by 'artist'?"
// Formula: base + (titleSim×0.10 + artistSim×0.05 + durScore×0.03) × acoustidScore, capped 1.0.
// acoustidScore is 1.0 for a direct MBZ MBID lookup, or the AcoustID match score for
// fingerprint-based matches.
func computeRecordingConfidence(base, acoustidScore float64, title, artist string, durationSecs int, fp *domain.Fingerprint) (float64, domain.MatchReasons) {
	reasons := domain.MatchReasons{Fingerprint: acoustidScore}

	titleSim := 0.0
	if t := fp.EmbeddedTags["title"]; t != "" && normalizeTitle(title) == normalizeTitle(t) {
		titleSim = 1.0
	}
	reasons.TitleTag = titleSim

	artistSim := 0.0
	if a := fp.EmbeddedTags["artist"]; a != "" && normalizeArtist(artist) == normalizeArtist(a) {
		artistSim = 1.0
	}
	reasons.ArtistTag = artistSim

	embeddedMS, _ := strconv.Atoi(fp.EmbeddedTags["duration_ms"])
	durScore := durationScore(embeddedMS, durationSecs)
	reasons.Duration = durScore

	bonus := (titleSim*bonusTitleAgreement + artistSim*bonusArtistAgreement + durScore*bonusDurationAgreement) * acoustidScore
	score := base + bonus
	if score > 1.0 {
		score = 1.0
	}
	return score, reasons
}

// computeReleaseConfidence returns the release-level confidence and signal contributions.
//
// Answers "Is this the specific edition we think it is?"
// Formula: albumTagSimilarity×0.80 + yearAgreement×0.20. Both weights sum to 1.0
// so rel_conf has true range [0, 1] and combinedConfidence can reach the
// shortCircuitThreshold of 0.85.
// When embeddedAlbum is empty, returns 0.50 (genuinely uncertain; sits below any
// confirmed match so release-level decisions require review).
func computeReleaseConfidence(embeddedAlbum string, embeddedYear int, releaseTitle, releaseDate string) (float64, domain.MatchReasons) {
	reasons := domain.MatchReasons{}
	if embeddedAlbum == "" {
		return 0.50, reasons
	}

	albumSim := albumTagSimilarity(embeddedAlbum, releaseTitle)
	reasons.AlbumTag = albumSim

	yearScore := 0.0
	if embeddedYear > 0 && releaseDate != "" {
		yearStr := strings.SplitN(releaseDate, "-", 2)[0]
		if releaseYear, err := strconv.Atoi(yearStr); err == nil && releaseYear > 0 && releaseYear == embeddedYear {
			yearScore = 1.0
		}
	}

	score := albumSim*0.80 + yearScore*0.20
	return score, reasons
}

// combinedConfidence weights recording identity (primary) and release precision (secondary).
// Confidence = 0.65×rec + 0.35×rel ensures no candidate can tie another that differs
// in either component — the 0.65/0.35 split is irrational relative to any simple score step.
func combinedConfidence(rec, rel float64) float64 {
	return 0.65*rec + 0.35*rel
}
