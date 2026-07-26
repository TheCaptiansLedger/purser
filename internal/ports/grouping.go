package ports

import (
	"context"
	"purser/internal/domain"
)

// GroupingResult is one path's grouping outcome: which identification unit
// it belongs to, and (if the grouping implementation detected a multi-disc
// structure) which disc. See docs/technical/pipeline-grouping-capability.md.
type GroupingResult struct {
	// GroupKey identifies the identification unit this path belongs to —
	// e.g. every track across an album's disc subfolders sharing one
	// GroupKey. Never domain.UnmatchedFile.ID: grouping runs before that ID
	// exists.
	GroupKey string

	// DiscNumber is the grouping implementation's structural guess at which
	// disc this path belongs to, 0 if not part of a detected multi-disc
	// structure. A guess, not a decision — a later fingerprinting step's
	// embedded tag always wins once it's actually read.
	DiscNumber int
}

// Grouping is a content-type-scoped capability: how discovered files
// combine into one identification unit differs by content type (an
// album's tracks share a folder; a movie file usually stands alone).
// Fanned out to by GroupingRegistry via ContentTypes(), per
// docs/adr/0002-solid-design-principles.md's OCP/ISP registry pattern —
// adding a new content type's grouping rule is a new Grouping
// implementation, never an edit to the registry or ScanExecutor.
//
// Batch-shaped deliberately: GroupKeys is called once per scan Job with
// every discovered path, not once per file, because multi-disc roll-up
// needs sibling visibility a single path in isolation can't provide.
type Grouping interface {
	// ContentTypes reports which domain.ContentType values this
	// implementation handles.
	ContentTypes() []domain.ContentType

	// GroupKeys returns every path's GroupingResult in one call.
	GroupKeys(ctx context.Context, paths []string) (map[string]GroupingResult, error)
}

// GroupingResolver is the fan-out dispatch capability ScanExecutor depends
// on: given a Job's already-resolved content type and every discovered
// path, look up the matching registered Grouping implementation (or fall
// back to identity grouping if none is registered for that content type)
// and return its result, in one batched call. Kept distinct from Grouping
// itself because a single Grouping implementation is scoped to the content
// types it declares and never sees a contentType argument — the resolver
// is what decides which one to call.
type GroupingResolver interface {
	GroupKeys(ctx context.Context, contentType domain.ContentType, paths []string) (map[string]GroupingResult, error)
}
