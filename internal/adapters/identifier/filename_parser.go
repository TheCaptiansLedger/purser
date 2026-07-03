package identifier

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseVideoFilename extracts title, year, season, and episode from a
// Sonarr/Radarr-style release filename (base name without extension).
// season and episode are 0 when not present (movie-style naming).
func ParseVideoFilename(name string) (title string, year int, season, episode int) {
	clean := cleanQualityTokens(name)

	// TV: SnnEnn or nnxnn pattern
	if s, e, rest := extractSE(clean); s > 0 {
		return normalizeTitle(rest), 0, s, e
	}

	// Movie: title (YYYY) or title.YYYY
	if t, y := extractTitleYear(clean); t != "" {
		return t, y, 0, 0
	}

	return normalizeTitle(clean), 0, 0, 0
}

// ParseAdultFilename extracts studio slug, title, performer names, a date, and a
// field count from a common adult release filename base. fieldCount is the number
// of recognised fields parsed (higher = more confident filename parse).
func ParseAdultFilename(name string) (studio, title string, performers []string, date time.Time, fieldCount int) {
	// Replace dots, underscores and dashes used as word separators with spaces.
	s := strings.NewReplacer(".", " ", "_", " ").Replace(name)

	// Try to find a date like 2024-03-15 or 20240315
	if d, rest := extractDate(s); !d.IsZero() {
		s = rest
		date = d
		fieldCount++
	}

	// First token that looks like a short uppercase slug is treated as the studio.
	parts := strings.Fields(s)
	if len(parts) > 0 && isStudioSlug(parts[0]) {
		studio = parts[0]
		parts = parts[1:]
		fieldCount++
	}

	// Tokens in [brackets] or after known separators are performer names.
	var titleParts []string
	for _, p := range parts {
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			performers = append(performers, strings.Trim(p, "[]"))
			fieldCount++
		} else {
			titleParts = append(titleParts, p)
		}
	}
	if len(performers) > 0 {
		fieldCount++
	}

	title = strings.TrimSpace(strings.Join(titleParts, " "))
	if title != "" {
		fieldCount++
	}
	return studio, title, performers, date, fieldCount
}

// ParseTrackFilename extracts track number and title from a filename base
// (no extension) like "01 - Bella Donna" or "03. Title Here".
func ParseTrackFilename(base string) (trackNum int, title string) {
	re := regexp.MustCompile(`^(\d{1,3})[.\s\-]+(.+)$`)
	m := re.FindStringSubmatch(strings.TrimSpace(base))
	if m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, strings.TrimSpace(m[2])
	}
	return 0, strings.TrimSpace(base)
}

// cleanQualityTokens removes common release quality/group tokens from a filename base.
var qualityPattern = regexp.MustCompile(
	`(?i)\b(1080p|720p|480p|2160p|4k|bluray|blu-ray|web-dl|webrip|hdtv|dvdrip|x264|x265|hevc|avc|aac|ac3|dts|hdr|hdr10|remux|proper|repack|\[.+?\]|\(.+?\))\b`,
)

func cleanQualityTokens(name string) string {
	clean := qualityPattern.ReplaceAllString(name, " ")
	return strings.Join(strings.Fields(clean), " ")
}

var (
	sePattern    = regexp.MustCompile(`(?i)[Ss](\d+)[Ee](\d+)`)
	altSEPattern = regexp.MustCompile(`(?i)(\d+)x(\d+)`)
)

func extractSE(name string) (season, episode int, rest string) {
	for _, re := range []*regexp.Regexp{sePattern, altSEPattern} {
		loc := re.FindStringIndex(name)
		if loc == nil {
			continue
		}
		m := re.FindStringSubmatch(name)
		s, _ := strconv.Atoi(m[1])
		e, _ := strconv.Atoi(m[2])
		before := strings.TrimSpace(name[:loc[0]])
		after := strings.TrimSpace(name[loc[1]:])
		rest = before
		if after != "" && before == "" {
			rest = after
		}
		return s, e, rest
	}
	return 0, 0, ""
}

var (
	yearInParens = regexp.MustCompile(`\((\d{4})\)`)
	yearBare     = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
)

func extractTitleYear(name string) (title string, year int) {
	if m := yearInParens.FindStringIndex(name); m != nil {
		y, _ := strconv.Atoi(yearInParens.FindStringSubmatch(name)[1])
		t := strings.TrimSpace(name[:m[0]])
		return normalizeTitle(t), y
	}
	if m := yearBare.FindStringIndex(name); m != nil {
		y, _ := strconv.Atoi(yearBare.FindStringSubmatch(name)[1])
		t := strings.TrimSpace(name[:m[0]])
		if t != "" {
			return normalizeTitle(t), y
		}
	}
	return "", 0
}

func normalizeTitle(s string) string {
	s = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(s)
	return strings.TrimSpace(s)
}

var datePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(\d{4})[.\-](\d{2})[.\-](\d{2})`),
	regexp.MustCompile(`(\d{4})(\d{2})(\d{2})`),
}

func extractDate(s string) (time.Time, string) {
	for _, re := range datePatterns {
		m := re.FindStringIndex(s)
		if m == nil {
			continue
		}
		parts := re.FindStringSubmatch(s)
		y, _ := strconv.Atoi(parts[1])
		mo, _ := strconv.Atoi(parts[2])
		d, _ := strconv.Atoi(parts[3])
		if mo < 1 || mo > 12 || d < 1 || d > 31 {
			continue
		}
		t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
		rest := strings.TrimSpace(s[:m[0]]) + " " + strings.TrimSpace(s[m[1]:])
		return t, strings.TrimSpace(rest)
	}
	return time.Time{}, s
}

// isStudioSlug returns true for short all-caps or all-lowercase tokens that
// resemble studio abbreviations (e.g. "SOD", "ideapocket", "MOODYZ").
func isStudioSlug(s string) bool {
	if len(s) < 2 || len(s) > 12 {
		return false
	}
	return s == strings.ToUpper(s) || s == strings.ToLower(s)
}
