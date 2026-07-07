package identifier

import (
	"context"
	"log/slog"
	"purser/internal/domain"
	"regexp"
	"sort"
	"strings"
)

var (
	rgSuffixParens   = regexp.MustCompile(`(?i)\s*\([^)]*(?:remaster(?:ed)?|deluxe|edition|explicit|clean|live|remix|version|anniversary|bonus)[^)]*\)`)
	rgSuffixBrackets = regexp.MustCompile(`(?i)\s*\[[^\]]*(?:remaster(?:ed)?|deluxe|edition|explicit|clean|live|remix|version|anniversary|bonus)[^\]]*\]`)
	rgSuffixDash     = regexp.MustCompile(`(?i)\s*-\s+\d{4}\s+(?:remaster(?:ed)?|mix|version|edition)\s*$`)
)

// StripAlbumSuffixes removes edition/variant qualifiers from an album title,
// preserving original casing. Strips parenthetical and bracketed phrases
// containing keywords like "Remaster", "Deluxe", "Edition", "Live",
// "Anniversary", or "Bonus". Also strips trailing dash-separated
// year+keyword suffixes (e.g. "- 2009 Mix"). Returns title unchanged when
// no suffix matches.
func StripAlbumSuffixes(title string) string {
	s := rgSuffixBrackets.ReplaceAllString(title, "")
	s = rgSuffixParens.ReplaceAllString(s, "")
	s = rgSuffixDash.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// ScoreBarcodeSignal returns 1.0 when tags.Barcode is non-empty and matches
// candidate.ReleaseBarcode; 0.0 otherwise.
func ScoreBarcodeSignal(ctx context.Context, root string, tags domain.MusicTagSummary, candidate domain.MusicReleaseCandidate) float64 {
	var score float64
	if tags.Barcode != "" && tags.Barcode == candidate.ReleaseBarcode {
		score = 1.0
	}
	slog.InfoContext(ctx, "rg signal", "root", root, "signal", "barcode", "score", score)
	return score
}

// ScoreISRCSignal returns the fraction of non-empty ISRCs in tags that appear
// in candidateISRCs. Returns 0.0 when tags has no non-empty ISRCs or
// candidateISRCs is empty.
func ScoreISRCSignal(ctx context.Context, root string, tags domain.MusicTagSummary, candidateISRCs []string) float64 {
	isrcSet := make(map[string]struct{}, len(candidateISRCs))
	for _, isrc := range candidateISRCs {
		if isrc != "" {
			isrcSet[strings.ToUpper(isrc)] = struct{}{}
		}
	}

	var present, matched int
	for _, isrc := range tags.ISRCs {
		if isrc == "" {
			continue
		}
		present++
		if _, ok := isrcSet[strings.ToUpper(isrc)]; ok {
			matched++
		}
	}

	var score float64
	if present > 0 {
		score = float64(matched) / float64(present)
	}
	slog.InfoContext(ctx, "rg signal", "root", root, "signal", "isrc", "score", score)
	return score
}

type rgNameScore struct {
	title string
	score float64
}

// ScoreFuzzyNameSignal returns the best name-similarity score between
// tags.AlbumTitle (after StripAlbumSuffixes) and each candidate's
// ReleaseGroupTitle (also suffix-stripped). Returns 0.0 when tags.AlbumTitle
// or candidates is empty.
func ScoreFuzzyNameSignal(ctx context.Context, root string, tags domain.MusicTagSummary, candidates []domain.MusicReleaseCandidate) float64 {
	if len(candidates) == 0 || tags.AlbumTitle == "" {
		slog.InfoContext(ctx, "rg signal", "root", root, "signal", "rgNameFuzzy", "score", 0.0)
		return 0.0
	}

	strippedLocal := StripAlbumSuffixes(tags.AlbumTitle)
	if strippedLocal != tags.AlbumTitle {
		slog.DebugContext(ctx, "suffix stripped", "original", tags.AlbumTitle, "stripped", strippedLocal)
	}

	scored := make([]rgNameScore, 0, len(candidates))
	for _, c := range candidates {
		strippedRG := StripAlbumSuffixes(c.ReleaseGroupTitle)
		sc := albumTagSimilarity(strippedLocal, strippedRG)
		scored = append(scored, rgNameScore{title: c.ReleaseGroupTitle, score: sc})
	}

	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })

	top := scored
	if len(top) > 3 {
		top = top[:3]
	}
	slog.DebugContext(ctx, "fuzzy candidates", "root", root, "top", top)

	best := scored[0].score
	slog.InfoContext(ctx, "rg signal", "root", root, "signal", "rgNameFuzzy", "score", best)
	return best
}

// FilterByTrackCount returns candidates whose ReleaseTrackCount matches
// tags.TotalTracks. Returns all candidates unmodified when tags.TotalTracks
// is 0 (unknown), or when every candidate would be eliminated.
func FilterByTrackCount(ctx context.Context, root string, tags domain.MusicTagSummary, candidates []domain.MusicReleaseCandidate) []domain.MusicReleaseCandidate {
	if tags.TotalTracks == 0 || len(candidates) == 0 {
		return candidates
	}

	var filtered []domain.MusicReleaseCandidate
	for _, c := range candidates {
		if c.ReleaseTrackCount == tags.TotalTracks {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) == 0 {
		return candidates
	}

	slog.InfoContext(ctx, "rg signal", "root", root, "signal", "trackCount",
		"score", float64(len(filtered))/float64(len(candidates)))
	return filtered
}
