package ports

import (
	"context"
	"purser/internal/domain"
)

// SidecarKind is a discovered file's classification relative to the
// identification pipeline. See
// docs/technical/pipeline-music-sidecar-classifier.md.
type SidecarKind string

const (
	// SidecarKindNone means the file is not a sidecar — it proceeds through
	// the pipeline normally (hash, fingerprint, identify).
	SidecarKindNone SidecarKind = ""

	// SidecarKindImage means the file is a cover-art candidate — excluded
	// from Tasks, handled later at persist time.
	SidecarKindImage SidecarKind = "image"

	// SidecarKindOther means the file is a companion asset with no
	// destination in Purser's domain model (.nfo/.cue/.log/.m3u/
	// unrecognized) — excluded from Tasks, ignored entirely.
	SidecarKindOther SidecarKind = "other"
)

// SidecarClassifier is a content-type-scoped capability: what counts as a
// sidecar differs by content type (subtitles matter for video, not music).
// Fanned out to by SidecarClassifierRegistry via ContentTypes(), per
// docs/adr/0002-solid-design-principles.md's OCP/ISP registry pattern —
// adding a new content type's classification rule is a new SidecarClassifier
// implementation, never an edit to the registry or ScanService.
//
// Per-path, unlike Grouping: classifying one file needs no sibling
// visibility (it's an extension/filename check), so it runs directly in
// ScanService.Trigger rather than batched in ScanExecutor. See
// docs/adr/0024-pipeline-core.md's "Sidecar/companion assets are classified
// before identification" decision.
type SidecarClassifier interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// Classify returns path's SidecarKind.
	Classify(ctx context.Context, path string) (SidecarKind, error)
}

// SidecarClassifierResolver is the fan-out dispatch capability ScanService
// depends on: given a Job's already-resolved content type and a discovered
// path, look up the matching registered SidecarClassifier implementation (or
// fall back to NoopClassifier if none is registered for that content type)
// and return its result. Kept distinct from SidecarClassifier itself because
// a single SidecarClassifier implementation is scoped to the content types
// it declares and never sees a contentType argument — the resolver is what
// decides which one to call.
type SidecarClassifierResolver interface {
	Classify(ctx context.Context, contentType domain.ContentType, path string) (SidecarKind, error)
}
