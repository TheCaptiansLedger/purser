package identifier

import (
	"purser/internal/domain"
	"sort"
	"strings"
)

// normalizeFingerprint returns a non-nil Fingerprint with a non-nil EmbeddedTags map.
func normalizeFingerprint(fp *domain.Fingerprint) *domain.Fingerprint {
	if fp == nil {
		return &domain.Fingerprint{EmbeddedTags: map[string]string{}}
	}
	if fp.EmbeddedTags == nil {
		fp.EmbeddedTags = map[string]string{}
	}
	return fp
}

// shortCircuitThreshold is the minimum confidence at which an identifier
// stops running further strategies in its chain. The scan service uses the
// same value as its auto-import threshold.
const shortCircuitThreshold = 0.85

// sortCandidates sorts candidates by descending confidence in place.
func sortCandidates(candidates []domain.MatchCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Confidence > candidates[j].Confidence
	})
}

// normalizeQuotes replaces Unicode typographic apostrophes and quotation marks
// with their ASCII equivalents so that embedded tag text (which uses U+0027)
// matches library titles sourced from MusicBrainz (which uses U+2019).
func normalizeQuotes(s string) string {
	s = strings.ReplaceAll(s, "’", "'")  // RIGHT SINGLE QUOTATION MARK
	s = strings.ReplaceAll(s, "‘", "'")  // LEFT SINGLE QUOTATION MARK
	s = strings.ReplaceAll(s, "“", "\"") // LEFT DOUBLE QUOTATION MARK
	s = strings.ReplaceAll(s, "”", "\"") // RIGHT DOUBLE QUOTATION MARK
	return s
}

// aboveThreshold reports whether any candidate meets or exceeds shortCircuitThreshold.
func aboveThreshold(candidates []domain.MatchCandidate) bool {
	for _, c := range candidates {
		if c.Confidence >= shortCircuitThreshold {
			return true
		}
	}
	return false
}

// anyItemLinked reports whether any candidate has a resolved library Item.
func anyItemLinked(candidates []domain.MatchCandidate) bool {
	for _, c := range candidates {
		if c.Item != nil {
			return true
		}
	}
	return false
}
