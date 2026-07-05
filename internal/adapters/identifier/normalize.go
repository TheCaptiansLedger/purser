package identifier

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var (
	reEditionParens   = regexp.MustCompile(`(?i)\s*\([^)]*(?:remaster(?:ed)?|deluxe|edition|explicit|clean|live|remix|version)[^)]*\)`)
	reEditionBrackets = regexp.MustCompile(`(?i)\s*\[[^\]]*(?:remaster(?:ed)?|deluxe|edition|explicit|clean|live|remix|version)[^\]]*\]`)
	reFeaturing       = regexp.MustCompile(`(?i)\s+(?:feat\.|ft\.|featuring)\s+.*$`)
	reParenBracket    = regexp.MustCompile(`[()\[\]]`)
	reMultiSpace      = regexp.MustCompile(`\s+`)
)

func normalizeTitle(s string) string {
	s = norm.NFC.String(s)
	s = strings.ToLower(s)
	s = reEditionParens.ReplaceAllString(s, "")
	s = reEditionBrackets.ReplaceAllString(s, "")
	s = reFeaturing.ReplaceAllString(s, "")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func normalizeArtist(s string) string {
	s = norm.NFC.String(s)
	s = strings.ToLower(s)
	for _, article := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(s, article) {
			s = s[len(article):]
			break
		}
	}
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func normalizeAlbum(s string) string {
	s = norm.NFC.String(s)
	s = strings.ToLower(s)
	s = reEditionParens.ReplaceAllString(s, "")
	s = reEditionBrackets.ReplaceAllString(s, "")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// albumTagSimilarity scores how well an embedded album tag matches a candidate
// release title. Uses albumTagNormalize (strips paren/bracket chars but keeps
// content) so "Hi Infidelity" is a word-boundary prefix of
// "Hi Infidelity (2024 Remaster)" → 0.85, not an exact match → 1.00.
func albumTagSimilarity(embedded, releaseTitle string) float64 {
	ne := albumTagNormalize(embedded)
	nr := albumTagNormalize(releaseTitle)
	if ne == "" || nr == "" {
		return 0.0
	}
	if ne == nr {
		return 1.00
	}
	if strings.HasPrefix(nr, ne) && len(nr) > len(ne) && nr[len(ne)] == ' ' {
		return 0.85
	}
	if strings.Contains(nr, ne) {
		return 0.60
	}
	return 0.00
}

// albumTagNormalize applies NFC, lowercase, removes paren/bracket characters
// (keeping their content), and collapses whitespace. Distinct from normalizeAlbum
// which strips entire qualifier phrases — here we preserve qualifier text so
// prefix and contains checks in albumTagSimilarity work correctly.
func albumTagNormalize(s string) string {
	s = norm.NFC.String(s)
	s = strings.ToLower(s)
	s = reParenBracket.ReplaceAllString(s, " ")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// durationScore returns a gradient confidence score for duration agreement
// between an embedded tag duration (milliseconds) and a candidate duration
// (seconds). Returns 0.0 when either value is absent.
func durationScore(embeddedMS, candidateSecs int) float64 {
	if embeddedMS <= 0 || candidateSecs <= 0 {
		return 0.0
	}
	diff := embeddedMS/1000 - candidateSecs
	if diff < 0 {
		diff = -diff
	}
	switch {
	case diff <= 1:
		return 1.0
	case diff <= 3:
		return 0.75
	case diff <= 5:
		return 0.50
	case diff <= 10:
		return 0.25
	default:
		return 0.0
	}
}
