package afterdark

import (
	"path/filepath"
	"regexp"
	"strings"
)

// javCodeSeparatedPattern matches a JAV product code where the letter
// prefix and digit run are separated by a hyphen or underscore — the
// common "SSIS-001"/"SSIS_001" release-naming shape. Deliberately excludes
// a bare space as a separator (unlike Music's delimiters) and requires a
// 3-digit-minimum run: ordinary scene titles frequently contain
// "<Word> <n>"-shaped substrings ("Scene 22", "Part 2"), and a space
// separator plus a short digit run is exactly that shape. The 3-6 letter /
// 3-5 digit bounds are a starting heuristic tuned against realistic
// filenames, not asserted to catch every JAV naming convention or reject
// every false positive (e.g. a title genuinely containing a
// hyphen-separated word-then-3-digit-number would still false-positive);
// see jav_code_parser_test.go for the samples it was validated against.
var javCodeSeparatedPattern = regexp.MustCompile(`\b([A-Za-z]{3,6})[-_](\d{3,5})\b`)

// javCodeConcatenatedPattern matches a JAV product code with no separator
// at all — e.g. "ssis00001", ThePornDB's own `sku` shape (see
// docs/technical/afterdark-data_model.md §2). Requires a longer digit run
// (4-6) than the separated form specifically to avoid false-positiving on
// ordinary title words that happen to end in a small number (e.g. "Part2").
var javCodeConcatenatedPattern = regexp.MustCompile(`\b([A-Za-z]{3,6})(\d{4,6})\b`)

// ExtractJAVCode is AD4's second filename guesser: a pure, best-effort JAV
// product-code extraction (`SSIS-001`-style) feeding ThePornDB's
// `GET /jav?parse=` (docs/technical/afterdark-data_model.md §2,
// ports.ThePornDBClient.ResolveJAVCode) — AD5's JAV-code identification
// tier. Unlike FilenameParser, this isn't a ports.FilenameParser
// implementation: it has exactly one shape of output (a single code
// string) and exactly one caller, AD5's Identifier in this same package —
// no other content type has a JAV-code concept, so there's no registry
// capability to declare. Never constructs or touches a
// domain.MatchCandidate; that's AD5's job.
//
// path is checked first (extension stripped); if no code-shaped substring
// is found there, the immediate parent folder name is checked as a
// fallback, for libraries that name the folder after the code and leave
// generic filenames underneath it. ok is false when nothing code-shaped
// was found in either place. The returned code is normalized to uppercase
// "PREFIX-DIGITS"; ThePornDB's endpoint is documented as tolerant of an
// imperfectly parsed code, so no further normalization (e.g. stripping
// leading zeros) is attempted here.
func ExtractJAVCode(path string) (code string, ok bool) {
	clean := filepath.Clean(path)
	leaf := stripSceneExtension(filepath.Base(clean))
	if code, ok := findJAVCode(leaf); ok {
		return code, true
	}

	parentName := filepath.Base(filepath.Dir(clean))
	return findJAVCode(parentName)
}

// findJAVCode searches name for a JAV-code-shaped substring, preferring the
// separated form (javCodeSeparatedPattern) over the stricter concatenated
// fallback (javCodeConcatenatedPattern).
func findJAVCode(name string) (code string, ok bool) {
	if m := javCodeSeparatedPattern.FindStringSubmatch(name); m != nil {
		return normalizeJAVCode(m[1], m[2]), true
	}
	if m := javCodeConcatenatedPattern.FindStringSubmatch(name); m != nil {
		return normalizeJAVCode(m[1], m[2]), true
	}
	return "", false
}

// normalizeJAVCode joins prefix and digits into ThePornDB's canonical
// "PREFIX-DIGITS" query shape, uppercasing the prefix.
func normalizeJAVCode(prefix, digits string) string {
	return strings.ToUpper(prefix) + "-" + digits
}
