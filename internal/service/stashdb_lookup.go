package service

import (
	"context"
	"purser/internal/ports"
)

// StashDBLookup exposes internal/adapters/stashdb's read-only performer
// lookup capability directly to a caller — see
// proto/purser/afterdark/v1/stashdb.proto's own doc comment for why this
// exists. Depends on ports.StashDBClient directly, the same deliberate,
// narrow exception to "services depend on ports describing a capability,
// not a specific provider" that FanartTVLookup/WikidataLookup already rely
// on.
type StashDBLookup struct {
	stashDB ports.StashDBClient
}

// NewStashDBLookup constructs a StashDBLookup backed by stashDB.
func NewStashDBLookup(stashDB ports.StashDBClient) *StashDBLookup {
	return &StashDBLookup{stashDB: stashDB}
}

// LookupPerformer fetches one performer by StashDB ID — a thin passthrough
// to ports.StashDBClient.LookupPerformer.
func (s *StashDBLookup) LookupPerformer(ctx context.Context, id string) (*ports.Performer, error) {
	return s.stashDB.LookupPerformer(ctx, id)
}

// SearchPerformers free-text searches performers by name/alias — a thin
// passthrough to ports.StashDBClient.SearchPerformers. The path a caller
// with no known StashDB ID for a Person uses (#703).
func (s *StashDBLookup) SearchPerformers(ctx context.Context, term string) ([]ports.Performer, error) {
	return s.stashDB.SearchPerformers(ctx, term)
}
