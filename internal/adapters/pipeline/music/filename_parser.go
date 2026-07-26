package music

import (
	"context"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"regexp"
	"strings"
)

// trailingYearPattern matches a trailing parenthesized or bracketed year
// annotation on a leaf folder name: "Hi Infidelity (1980)" and
// "Hi Infidelity [1980]" both strip to "Hi Infidelity". See
// docs/technical/pipeline-music-filename-parser.md.
var trailingYearPattern = regexp.MustCompile(`\s*[(\[](?:19|20)\d{2}[)\]]\s*$`)

// purelyNumericPattern matches a leaf/parent name that's nothing but
// digits — junk per the blocklist rule below.
var purelyNumericPattern = regexp.MustCompile(`^\d+$`)

// junkNames is the starting blocklist of folder names that never carry
// artist/album signal — a starting list, not asserted complete; expected
// to need tuning against real libraries, per
// docs/technical/pipeline-music-filename-parser.md. Compared
// case-insensitively.
var junkNames = map[string]struct{}{
	"music":      {},
	"downloads":  {},
	"new folder": {},
}

// minMeaningfulNameLength is the shortest a name can be and still be
// considered usable signal rather than junk.
const minMeaningfulNameLength = 2

// delimiters are checked in order; the first one occurring in a leaf name
// splits it into artist/album, per
// docs/technical/pipeline-music-filename-parser.md.
var delimiters = []string{" - ", " – ", "_-_"}

// FilenameParser implements ports.FilenameParser for domain.ContentTypeMusic:
// a pure, best-effort (artist, album) guess from a group's folder path,
// used only as candidate-generation's last-resort fallback when
// tag-derived generation finds nothing
// (docs/adr/0025-music-identification-confidence-scoring.md). Never
// constructs or touches a domain.MatchCandidate — see
// docs/technical/pipeline-music-filename-parser.md.
type FilenameParser struct{}

var _ ports.FilenameParser = FilenameParser{}

// ContentTypes implements ports.FilenameParser.
func (FilenameParser) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// Parse implements ports.FilenameParser, per
// docs/technical/pipeline-music-filename-parser.md's "Algorithm" section:
// strip a trailing year annotation, reject junk names, split on a
// delimiter when present, otherwise fall back to leaf=album/parent=artist
// (skipping the parent when it's scanRoot itself).
func (FilenameParser) Parse(_ context.Context, groupPath, scanRoot string) (artist, album string, ok bool) {
	clean := filepath.Clean(groupPath)
	leaf := stripTrailingYear(filepath.Base(clean))
	if isJunkName(leaf) {
		return "", "", false
	}

	if a, b, found := splitOnDelimiter(leaf); found {
		return a, b, true
	}

	album = leaf
	parentDir := filepath.Dir(clean)
	if parentDir != filepath.Clean(scanRoot) {
		if parentName := filepath.Base(parentDir); !isJunkName(parentName) {
			artist = parentName
		}
	}
	return artist, album, true
}

// stripTrailingYear removes a trailing "(YYYY)"/"[YYYY]" annotation from
// name, if present.
func stripTrailingYear(name string) string {
	return trailingYearPattern.ReplaceAllString(name, "")
}

// isJunkName reports whether name carries no artist/album signal: blank,
// too short, purely numeric, or on the junkNames blocklist
// (case-insensitive).
func isJunkName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return true
	}
	if _, blocked := junkNames[strings.ToLower(trimmed)]; blocked {
		return true
	}
	if purelyNumericPattern.MatchString(trimmed) {
		return true
	}
	return len(trimmed) < minMeaningfulNameLength
}

// splitOnDelimiter splits name on the first delimiter (checked in
// delimiters' order) found within it, returning the trimmed parts before
// and after. found is false if name contains none of them.
func splitOnDelimiter(name string) (before, after string, found bool) {
	for _, delim := range delimiters {
		if idx := strings.Index(name, delim); idx >= 0 {
			return strings.TrimSpace(name[:idx]), strings.TrimSpace(name[idx+len(delim):]), true
		}
	}
	return "", "", false
}
