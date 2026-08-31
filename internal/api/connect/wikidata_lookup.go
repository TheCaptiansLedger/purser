package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// wikidataLookupService is the narrow interface WikidataHandler depends
// on — see personService for the DIP convention this follows.
type wikidataLookupService interface {
	LookupImage(ctx context.Context, entityURL string) ([]ports.WikidataImage, error)
}

// WikidataHandler implements musicv1connect.WikidataServiceHandler — see
// proto/purser/music/v1/wikidata.proto's own doc comment for why this
// exists.
type WikidataHandler struct {
	musicv1connect.UnimplementedWikidataServiceHandler
	svc    wikidataLookupService
	logger *slog.Logger
}

// NewWikidataHandler constructs a WikidataHandler backed by svc.
func NewWikidataHandler(svc wikidataLookupService, logger *slog.Logger) *WikidataHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WikidataHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "WikidataService")}
}

// LookupImage implements musicv1connect.WikidataServiceHandler.
func (h *WikidataHandler) LookupImage(ctx context.Context, req *connect.Request[musicv1.LookupWikidataImageRequest]) (*connect.Response[musicv1.LookupWikidataImageResponse], error) {
	images, err := h.svc.LookupImage(ctx, req.Msg.GetUrl())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	out := make([]*musicv1.WikidataImage, len(images))
	for i, img := range images {
		out[i] = &musicv1.WikidataImage{Url: img.URL}
	}
	return connect.NewResponse(&musicv1.LookupWikidataImageResponse{Images: out}), nil
}
