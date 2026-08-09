package afterdark

import (
	"context"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"unicode"
)

// videoExtensions is the single source of truth for "is this file a scene" —
// known video extensions classify as ports.SidecarKindNone (proceeds through
// the pipeline normally, becomes a Task). See
// docs/adr/0024-pipeline-core.md's "Sidecar/companion assets are classified
// before identification" decision.
var videoExtensions = map[string]struct{}{
	".mp4":  {},
	".mkv":  {},
	".avi":  {},
	".wmv":  {},
	".mov":  {},
	".m4v":  {},
	".webm": {},
	".flv":  {},
	".ts":   {},
	".m2ts": {},
}

// imageExtensions classify as ports.SidecarKindImage — poster/screenshot
// candidates, excluded from Tasks. Unlike Music's M10 cover-art step,
// AD9's image attachment comes from AD1/AD2's provider lookup responses,
// not a local-disk walk, so classification here only keeps these files out
// of the Task/identification path — nothing later re-reads them by
// filename convention.
var imageExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".webp": {},
	".gif":  {},
}

// SidecarClassifier implements ports.SidecarClassifier for
// domain.ContentTypeAdult: known video extensions classify as
// ports.SidecarKindNone, image extensions as ports.SidecarKindImage, and
// everything else (.nfo, unrecognized) as ports.SidecarKindOther — same
// three-way shape as pipelinemusic.SidecarClassifier. The one AfterDark-
// specific wrinkle is trailers: a trailer file carries a video extension
// (it plays back the same as a scene) but is a companion asset, not its
// own identification unit, so the trailer check runs before the video-
// extension check below.
type SidecarClassifier struct{}

var _ ports.SidecarClassifier = SidecarClassifier{}

// ContentTypes implements ports.SidecarClassifier.
func (SidecarClassifier) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

// Classify implements ports.SidecarClassifier.
func (SidecarClassifier) Classify(_ context.Context, path string) (ports.SidecarKind, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := imageExtensions[ext]; ok {
		return ports.SidecarKindImage, nil
	}
	if isTrailer(path) {
		return ports.SidecarKindOther, nil
	}
	if _, ok := videoExtensions[ext]; ok {
		return ports.SidecarKindNone, nil
	}
	return ports.SidecarKindOther, nil
}

// isTrailer reports whether path's filename stem contains "trailer" as a
// whole token (e.g. "Scene-Trailer.mp4", "trailer.mp4"), not merely as a
// substring (e.g. "Blazing Trailers.mp4" does not match). Checked ahead of
// the video-extension rule since a trailer file's extension alone is
// indistinguishable from a scene's.
func isTrailer(path string) bool {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	tokens := strings.FieldsFunc(stem, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, tok := range tokens {
		if strings.EqualFold(tok, "trailer") {
			return true
		}
	}
	return false
}
