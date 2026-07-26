// Package music holds Music's content-type-specific implementations of the
// pipeline's fanned-out capabilities (grouping, fingerprinting, more to
// come) — adapter-layer, per docs/adr/0001-hexagonal-architecture.md's
// "content-type-specific behavior lives in adapter implementations" rule.
// Named to match the internal/adapters/store/music precedent.
package music

import (
	"context"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"regexp"
	"strconv"
	"strings"
)

// discFolderPattern matches a disc/LP subfolder name in its entirety:
// CD1, CD 1, Disc1, Disc 2, D3, LP1, LP 2 (case-insensitive, optional space
// before the digit run). See docs/technical/pipeline-grouping-capability.md.
var discFolderPattern = regexp.MustCompile(`(?i)^(?:cd|disc|d|lp)\s*(\d+)$`)

// Grouping implements ports.Grouping for domain.ContentTypeMusic: default
// one-folder-one-group (GroupKey = immediate parent directory), with a
// multi-disc/multi-LP roll-up override when a folder's only children that
// contain a discovered audio file are all disc-pattern-named subfolders —
// GroupKey then becomes the grandparent and DiscNumber is the matched
// digit. Inconsistent/ambiguous siblings are left split, never merged: a
// wrong split is recoverable at review, a wrong merge isn't. Purely
// structural — no tag reading; see
// docs/technical/pipeline-grouping-capability.md.
type Grouping struct{}

var _ ports.Grouping = Grouping{}

// ContentTypes implements ports.Grouping.
func (Grouping) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// GroupKeys implements ports.Grouping.
func (Grouping) GroupKeys(_ context.Context, paths []string) (map[string]ports.GroupingResult, error) {
	parentOf := make(map[string]string, len(paths))
	childDirsByGrandparent := make(map[string][]string)
	dirSeen := make(map[string]struct{})
	for _, p := range paths {
		dir := filepath.Dir(p)
		parentOf[p] = dir
		if _, ok := dirSeen[dir]; ok {
			continue
		}
		dirSeen[dir] = struct{}{}
		grandparent := filepath.Dir(dir)
		childDirsByGrandparent[grandparent] = append(childDirsByGrandparent[grandparent], dir)
	}

	discNumberByDir := make(map[string]int, len(dirSeen))
	for dir := range dirSeen {
		digit, ok := matchDiscFolder(filepath.Base(dir))
		if !ok {
			continue
		}
		grandparent := filepath.Dir(dir)
		if !allSiblingsMatchDiscPattern(childDirsByGrandparent[grandparent]) {
			continue
		}
		discNumberByDir[dir] = digit
	}

	result := make(map[string]ports.GroupingResult, len(paths))
	for _, p := range paths {
		dir := parentOf[p]
		if discNumber, ok := discNumberByDir[dir]; ok {
			result[p] = ports.GroupingResult{GroupKey: filepath.Dir(dir), DiscNumber: discNumber}
			continue
		}
		result[p] = ports.GroupingResult{GroupKey: dir, DiscNumber: 0}
	}
	return result, nil
}

// allSiblingsMatchDiscPattern reports whether every directory in dirs (all
// audio-bearing children of one grandparent) matches discFolderPattern.
func allSiblingsMatchDiscPattern(dirs []string) bool {
	for _, dir := range dirs {
		if _, ok := matchDiscFolder(filepath.Base(dir)); !ok {
			return false
		}
	}
	return true
}

// matchDiscFolder reports whether name is a disc/LP folder name, and if so
// the digit extracted from it.
func matchDiscFolder(name string) (int, bool) {
	m := discFolderPattern.FindStringSubmatch(strings.TrimSpace(name))
	if m == nil {
		return 0, false
	}
	digit, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return digit, true
}
