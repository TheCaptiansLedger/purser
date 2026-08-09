package afterdark

import (
	"context"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"regexp"
	"strings"
)

// scenePurelyNumericPattern matches a leaf/parent name that's nothing but
// digits — junk per the blocklist rule below, same category as Music's
// FilenameParser.
var scenePurelyNumericPattern = regexp.MustCompile(`^\d+$`)

// sceneJunkNames is the starting blocklist of filename/folder leaf names
// that never carry studio/title signal — a starting list, not asserted
// complete; expected to need tuning against real libraries, same caveat
// Music's junkNames carries.
var sceneJunkNames = map[string]struct{}{
	"video":      {},
	"videos":     {},
	"downloads":  {},
	"new folder": {},
	"unsorted":   {},
}

// minSceneNameLength is the shortest a name can be and still be considered
// usable signal rather than junk.
const minSceneNameLength = 2

// sceneDelimiters are checked in order; the first one occurring in a leaf
// name splits it into studio/title, same convention as Music's delimiters.
var sceneDelimiters = []string{" - ", " – ", "_-_"}

// FilenameParser implements ports.FilenameParser for
// domain.ContentTypeAdult: a pure, best-effort (studio, title) guess from a
// group's path, used as the AfterDark Identifier's (AD5) fuzzy-tier
// free-text search input — see docs/adr/0025-music-identification-
// confidence-scoring.md for the shape this mirrors (Music's M6) and
// docs/technical/afterdark-data_model.md §5.2 for AfterDark's own
// candidate-generation design. Never constructs or touches a
// domain.MatchCandidate.
//
// Unlike Music, AfterDark reuses IdentityGrouping as-is (one file, one
// group — see fingerprinter.go), so groupPath here is a *file* path, not a
// folder path: Parse strips the extension from the leaf before applying
// Music's M6-shaped algorithm (junk rejection, delimiter split, else
// leaf=title/parent=studio).
type FilenameParser struct{}

var _ ports.FilenameParser = FilenameParser{}

// ContentTypes implements ports.FilenameParser.
func (FilenameParser) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

// Parse implements ports.FilenameParser: strips groupPath's file extension
// to get the leaf, rejects junk leaves, splits on a delimiter when present,
// otherwise falls back to leaf=title/parent=studio (skipping the parent
// when it's scanRoot itself or itself junk).
func (FilenameParser) Parse(_ context.Context, groupPath, scanRoot string) (studio, title string, ok bool) {
	clean := filepath.Clean(groupPath)
	leaf := stripSceneExtension(filepath.Base(clean))
	if isSceneJunkName(leaf) {
		return "", "", false
	}

	if a, b, found := splitSceneOnDelimiter(leaf); found {
		return a, b, true
	}

	title = leaf
	parentDir := filepath.Dir(clean)
	if parentDir != filepath.Clean(scanRoot) {
		if parentName := filepath.Base(parentDir); !isSceneJunkName(parentName) {
			studio = parentName
		}
	}
	return studio, title, true
}

// stripSceneExtension removes name's file extension, if any — the one step
// Music's folder-leaf algorithm doesn't need, since AfterDark's groupPath is
// a file path (per IdentityGrouping), not a folder path.
func stripSceneExtension(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// isSceneJunkName reports whether name carries no studio/title signal:
// blank, too short, purely numeric, or on the sceneJunkNames blocklist
// (case-insensitive).
func isSceneJunkName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return true
	}
	if _, blocked := sceneJunkNames[strings.ToLower(trimmed)]; blocked {
		return true
	}
	if scenePurelyNumericPattern.MatchString(trimmed) {
		return true
	}
	return len(trimmed) < minSceneNameLength
}

// splitSceneOnDelimiter splits name on the first delimiter (checked in
// sceneDelimiters' order) found within it, returning the trimmed parts
// before and after. found is false if name contains none of them.
func splitSceneOnDelimiter(name string) (before, after string, found bool) {
	for _, delim := range sceneDelimiters {
		if idx := strings.Index(name, delim); idx >= 0 {
			return strings.TrimSpace(name[:idx]), strings.TrimSpace(name[idx+len(delim):]), true
		}
	}
	return "", "", false
}
