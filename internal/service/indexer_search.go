package service

import (
	"context"
	"purser/internal/ports"
)

// IndexerSearch exposes internal/adapters/prowlarr's (or any future
// IndexerSearcher adapter's) search capability directly to a caller — see
// proto/purser/acquisition/v1/indexer.proto's own doc comment for why this
// exists. Depends only on ports.IndexerSearcher, per
// docs/adr/0011-api-design.md's no-God-service rule — this is a one-port,
// one-method service, not something a future DownloadService (#585) should
// ever be folded into.
type IndexerSearch struct {
	searcher ports.IndexerSearcher
}

// NewIndexerSearch constructs an IndexerSearch backed by searcher.
func NewIndexerSearch(searcher ports.IndexerSearcher) *IndexerSearch {
	return &IndexerSearch{searcher: searcher}
}

// Search free-text searches across every indexer the configured backend
// has enabled — a thin passthrough to ports.IndexerSearcher.Search. A
// zero-result search is a valid, non-error empty slice (see that port's
// own doc comment), never an error here either.
func (s *IndexerSearch) Search(ctx context.Context, params ports.IndexerSearchParams) ([]ports.IndexerRelease, error) {
	return s.searcher.Search(ctx, params)
}
