package apiconnect

import (
	"context"
	"log/slog"
	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
	"purser/gen/go/purser/acquisition/v1/acquisitionv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"
)

// indexerSearchService is the narrow interface IndexerSearchHandler
// depends on — see personService for the DIP convention this follows.
type indexerSearchService interface {
	Search(ctx context.Context, params ports.IndexerSearchParams) ([]ports.IndexerRelease, error)
}

// IndexerSearchHandler implements acquisitionv1connect.IndexerServiceHandler
// — see proto/purser/acquisition/v1/indexer.proto's own doc comment for why
// this exists.
type IndexerSearchHandler struct {
	acquisitionv1connect.UnimplementedIndexerServiceHandler
	svc    indexerSearchService
	logger *slog.Logger
}

// NewIndexerSearchHandler constructs an IndexerSearchHandler backed by svc.
func NewIndexerSearchHandler(svc indexerSearchService, logger *slog.Logger) *IndexerSearchHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &IndexerSearchHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "IndexerService")}
}

// Search implements acquisitionv1connect.IndexerServiceHandler.
func (h *IndexerSearchHandler) Search(ctx context.Context, req *connect.Request[acquisitionv1.SearchIndexersRequest]) (*connect.Response[acquisitionv1.SearchIndexersResponse], error) {
	params := ports.IndexerSearchParams{
		Query:      req.Msg.GetQuery(),
		Categories: int32SliceToInt(req.Msg.GetCategories()),
		IndexerIDs: int32SliceToInt(req.Msg.GetIndexerIds()),
	}
	releases, err := h.svc.Search(ctx, params)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*acquisitionv1.IndexerRelease, len(releases))
	for i, r := range releases {
		out[i] = indexerReleaseToProto(r)
	}
	return connect.NewResponse(&acquisitionv1.SearchIndexersResponse{Releases: out}), nil
}

// int32SliceToInt converts a proto repeated int32 field to []int —
// ports.IndexerSearchParams.Categories/IndexerIDs are opaque backend ids
// (see that type's own doc comment), plain int on the port side since
// nothing in internal/ports needs proto's fixed width.
func int32SliceToInt(vs []int32) []int {
	if len(vs) == 0 {
		return nil
	}
	out := make([]int, len(vs))
	for i, v := range vs {
		out[i] = int(v)
	}
	return out
}
