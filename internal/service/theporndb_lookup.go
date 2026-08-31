package service

import (
	"context"
	"purser/internal/ports"
)

// ThePornDBLookup exposes internal/adapters/theporndb's read-only
// performer lookup capability directly to a caller — see
// proto/purser/afterdark/v1/theporndb.proto's own doc comment for why this
// exists. Depends on ports.ThePornDBClient directly, the same deliberate,
// narrow exception to "services depend on ports describing a capability,
// not a specific provider" that StashDBLookup/FanartTVLookup already rely
// on.
type ThePornDBLookup struct {
	tpdb ports.ThePornDBClient
}

// NewThePornDBLookup constructs a ThePornDBLookup backed by tpdb.
func NewThePornDBLookup(tpdb ports.ThePornDBClient) *ThePornDBLookup {
	return &ThePornDBLookup{tpdb: tpdb}
}

// LookupPerformer fetches one performer by ThePornDB UUID — a thin
// passthrough to ports.ThePornDBClient.LookupPerformer.
func (s *ThePornDBLookup) LookupPerformer(ctx context.Context, id string) (*ports.TPDBPerformer, error) {
	return s.tpdb.LookupPerformer(ctx, id)
}

// SearchPerformers free-text searches performers by name/alias — a thin
// passthrough to ports.ThePornDBClient.SearchPerformers. The path a caller
// with no known ThePornDB ID for a Person uses (#703).
func (s *ThePornDBLookup) SearchPerformers(ctx context.Context, term string) ([]ports.TPDBPerformer, error) {
	return s.tpdb.SearchPerformers(ctx, term)
}
