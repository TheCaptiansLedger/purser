package afterdark

import (
	"path/filepath"
	"regexp"
	"strings"
)

// uuidPattern matches a standard 8-4-4-4-12 hex UUID, case-insensitive —
// the ID shape both StashDB and ThePornDB use for every entity (confirmed
// live for ThePornDB via ports.ThePornDBClient's own "by ThePornDB UUID"
// doc comments; StashDB follows stash-box's same UUID-keyed schema). Many
// scrapers/downloader tools name a saved file — or its parent folder —
// after the source site's own scene UUID; that's this package's "file
// already carries a known external ID" direct-ID short-circuit (AD5's
// Identifier), distinct from AD4's ExtractJAVCode (a product code, not a
// provider-native ID) and from AD3b's computed OSHash/PHash (derived from
// the file's bytes, not carried by it).
var uuidPattern = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)

// extractUUID checks path's leaf (extension stripped) first, falling back
// to the immediate parent folder name — the same two-step shape
// ExtractJAVCode uses. ok is false when no UUID-shaped substring is found
// in either place. The returned id is lowercased: both providers' own IDs
// are lowercase, and a filename-cased UUID needs normalizing before use as
// a lookup key.
func extractUUID(path string) (id string, ok bool) {
	clean := filepath.Clean(path)
	leaf := stripSceneExtension(filepath.Base(clean))
	if m := uuidPattern.FindString(leaf); m != "" {
		return strings.ToLower(m), true
	}

	parentName := filepath.Base(filepath.Dir(clean))
	if m := uuidPattern.FindString(parentName); m != "" {
		return strings.ToLower(m), true
	}
	return "", false
}
