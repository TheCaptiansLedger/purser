package fswatch

import (
	"path/filepath"
	"strings"
)

// UnitResolver decides which directory a changed path should be coalesced
// and reported against — the "unit" of change a caller cares about (e.g.
// an Album directory, not each track file inside it). Implementations
// must be pure functions of (root, path): Watcher calls UnitFor from its
// single internal goroutine, but it must be safe to construct once and
// reuse across every call.
type UnitResolver interface {
	// UnitFor returns the unit path for path, which is always root or a
	// descendant of root. ok is false if path should be ignored entirely
	// — not counted as activity, not reported.
	UnitFor(root, path string) (unit string, ok bool)
}

// DepthResolver treats the directory Depth path segments below root as
// the reportable unit. A path deeper than Depth rolls up into its
// ancestor at that depth — e.g. with Depth=2 on root/Artist/Album/Disc 1/
// track.flac, the unit is root/Artist/Album, so a multi-disc subfolder or
// a sidecar file dropped straight into the album directory both report
// against the same unit. A path shallower than Depth (nothing yet exists
// at the configured unit level) is ignored: DepthResolver deliberately
// does not report on intermediate directories like a bare "Artist" folder
// with no album underneath it yet.
//
// Depth 0 means root itself is the only unit — every change anywhere
// under root coalesces into one event.
type DepthResolver struct {
	Depth int
}

// UnitFor implements UnitResolver.
func (d DepthResolver) UnitFor(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return root, d.Depth == 0
	}

	slashRel := filepath.ToSlash(rel)
	if slashRel == ".." || strings.HasPrefix(slashRel, "../") {
		return "", false // outside root entirely
	}

	parts := strings.Split(slashRel, "/")
	if len(parts) < d.Depth {
		return "", false // shallower than the configured unit depth
	}

	segments := append([]string{root}, parts[:d.Depth]...)
	return filepath.Join(segments...), true
}
