package music

import (
	"context"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
)

// audioExtensions is the single source of truth for "is this file audio" —
// known audio extensions classify as ports.SidecarKindNone (proceeds through
// the pipeline normally). See
// docs/technical/pipeline-music-sidecar-classifier.md.
var audioExtensions = map[string]struct{}{
	".flac": {},
	".mp3":  {},
	".m4a":  {},
	".ogg":  {},
	".wav":  {},
	".aac":  {},
	".wma":  {},
	".opus": {},
}

// imageExtensions classify as ports.SidecarKindImage — cover-art candidates,
// excluded from Tasks and handled later at persist time. Filename
// convention (cover.*, folder.*, ...) isn't checked here; that ranking
// happens at attachment time, not classification time.
var imageExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".webp": {},
	".gif":  {},
}

// SidecarClassifier implements ports.SidecarClassifier for
// domain.ContentTypeMusic: known audio extensions classify as
// ports.SidecarKindNone, image extensions as ports.SidecarKindImage,
// everything else (.nfo/.cue/.log/.m3u/unrecognized) as
// ports.SidecarKindOther. See
// docs/technical/pipeline-music-sidecar-classifier.md.
type SidecarClassifier struct{}

var _ ports.SidecarClassifier = SidecarClassifier{}

// ContentTypes implements ports.SidecarClassifier.
func (SidecarClassifier) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// Classify implements ports.SidecarClassifier.
func (SidecarClassifier) Classify(_ context.Context, path string) (ports.SidecarKind, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := audioExtensions[ext]; ok {
		return ports.SidecarKindNone, nil
	}
	if _, ok := imageExtensions[ext]; ok {
		return ports.SidecarKindImage, nil
	}
	return ports.SidecarKindOther, nil
}
