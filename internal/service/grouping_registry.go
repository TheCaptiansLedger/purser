package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// GroupingRegistry implements ports.GroupingResolver by dispatching to
// whichever registered ports.Grouping declares the requested
// domain.ContentType via ContentTypes(), falling back to IdentityGrouping
// when none is registered — the same ContentTypes()-fan-out registry
// pattern documented in docs/adr/0024-pipeline-core.md's "Grouping is a
// pluggable, fanned-out capability" section. Adding a new content type's
// grouping rule means constructing NewGroupingRegistry with one more
// implementation at the composition root, never editing this type.
type GroupingRegistry struct {
	byContentType map[domain.ContentType]ports.Grouping
}

var _ ports.GroupingResolver = (*GroupingRegistry)(nil)

// NewGroupingRegistry builds a GroupingRegistry from groupings, indexing
// each by every domain.ContentType it declares via ContentTypes(). A later
// entry declaring a ContentType already claimed by an earlier one
// overwrites it — construction order matters only in that (deliberately
// unlikely) collision case.
func NewGroupingRegistry(groupings ...ports.Grouping) *GroupingRegistry {
	byContentType := make(map[domain.ContentType]ports.Grouping, len(groupings))
	for _, g := range groupings {
		for _, ct := range g.ContentTypes() {
			byContentType[ct] = g
		}
	}
	return &GroupingRegistry{byContentType: byContentType}
}

// GroupKeys implements ports.GroupingResolver.
func (r *GroupingRegistry) GroupKeys(ctx context.Context, contentType domain.ContentType, paths []string) (map[string]ports.GroupingResult, error) {
	g, ok := r.byContentType[contentType]
	if !ok {
		g = IdentityGrouping{}
	}
	return g.GroupKeys(ctx, paths)
}

// IdentityGrouping is the default ports.Grouping implementation:
// GroupingRegistry's fallback when no implementation is registered for a
// content type. Every path maps to {GroupKey: itself, DiscNumber: 0} — one
// file, one group, no roll-up. Deliberately not registered under any
// domain.ContentType itself (ContentTypes returns nil); GroupingRegistry
// falls back to it directly rather than looking it up by content type.
type IdentityGrouping struct{}

var _ ports.Grouping = IdentityGrouping{}

// ContentTypes implements ports.Grouping. IdentityGrouping is never looked
// up by content type — GroupingRegistry falls back to it directly — so
// this returns nil.
func (IdentityGrouping) ContentTypes() []domain.ContentType { return nil }

// GroupKeys implements ports.Grouping: every path maps to {GroupKey:
// itself, DiscNumber: 0}.
func (IdentityGrouping) GroupKeys(_ context.Context, paths []string) (map[string]ports.GroupingResult, error) {
	result := make(map[string]ports.GroupingResult, len(paths))
	for _, p := range paths {
		result[p] = ports.GroupingResult{GroupKey: p, DiscNumber: 0}
	}
	return result, nil
}
