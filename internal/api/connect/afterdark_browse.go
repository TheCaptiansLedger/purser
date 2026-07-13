package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// browseService is the narrow interface BrowseHandler depends on — see
// personService for the DIP convention this follows. It matches
// internal/service.AfterDarkBrowseService's method set exactly, but this
// handler depends on the interface, never the concrete service type, per
// docs/adr/0002-solid-design-principles.md's DIP rule.
type browseService interface {
	ListScenesInNetwork(ctx context.Context, networkID string, pageSize int, pageToken string) ([]*domain.Item, string, error)
	ListScenesForPerformer(ctx context.Context, personID string, pageSize int, pageToken string) ([]*domain.Item, string, error)
	ListPerformersForScene(ctx context.Context, itemID string, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error)
	ListPerformersForStudio(ctx context.Context, libraryEntryID string, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error)
	ListPerformersForNetwork(ctx context.Context, networkID string, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error)
	ListPerformers(ctx context.Context, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error)
}

// BrowseHandler implements afterdarkv1connect.BrowseServiceHandler —
// AfterDark's composing-service driving adapter, backed by
// internal/service.AfterDarkBrowseService. See
// docs/adr/0015-deletion-impact-and-composing-services.md.
type BrowseHandler struct {
	afterdarkv1connect.UnimplementedBrowseServiceHandler
	svc    browseService
	logger *slog.Logger
}

// NewBrowseHandler constructs a BrowseHandler backed by svc.
func NewBrowseHandler(svc browseService, logger *slog.Logger) *BrowseHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &BrowseHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "BrowseService")}
}

// ListScenesInNetwork implements afterdarkv1connect.BrowseServiceHandler.
func (h *BrowseHandler) ListScenesInNetwork(ctx context.Context, req *connect.Request[afterdarkv1.ListScenesInNetworkRequest]) (*connect.Response[afterdarkv1.ListScenesInNetworkResponse], error) {
	scenes, next, err := h.svc.ListScenesInNetwork(ctx, req.Msg.GetNetworkId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.ListScenesInNetworkResponse{Scenes: itemsToProto(scenes), NextPageToken: next}), nil
}

// ListScenesForPerformer implements afterdarkv1connect.BrowseServiceHandler.
func (h *BrowseHandler) ListScenesForPerformer(ctx context.Context, req *connect.Request[afterdarkv1.ListScenesForPerformerRequest]) (*connect.Response[afterdarkv1.ListScenesForPerformerResponse], error) {
	scenes, next, err := h.svc.ListScenesForPerformer(ctx, req.Msg.GetPersonId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.ListScenesForPerformerResponse{Scenes: itemsToProto(scenes), NextPageToken: next}), nil
}

// ListPerformersForScene implements afterdarkv1connect.BrowseServiceHandler.
func (h *BrowseHandler) ListPerformersForScene(ctx context.Context, req *connect.Request[afterdarkv1.ListPerformersForSceneRequest]) (*connect.Response[afterdarkv1.ListPerformersForSceneResponse], error) {
	views, next, err := h.svc.ListPerformersForScene(ctx, req.Msg.GetItemId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.ListPerformersForSceneResponse{Performers: performerViewsToProto(views), NextPageToken: next}), nil
}

// ListPerformersForStudio implements afterdarkv1connect.BrowseServiceHandler.
func (h *BrowseHandler) ListPerformersForStudio(ctx context.Context, req *connect.Request[afterdarkv1.ListPerformersForStudioRequest]) (*connect.Response[afterdarkv1.ListPerformersForStudioResponse], error) {
	views, next, err := h.svc.ListPerformersForStudio(ctx, req.Msg.GetLibraryEntryId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.ListPerformersForStudioResponse{Performers: performerViewsToProto(views), NextPageToken: next}), nil
}

// ListPerformersForNetwork implements afterdarkv1connect.BrowseServiceHandler.
func (h *BrowseHandler) ListPerformersForNetwork(ctx context.Context, req *connect.Request[afterdarkv1.ListPerformersForNetworkRequest]) (*connect.Response[afterdarkv1.ListPerformersForNetworkResponse], error) {
	views, next, err := h.svc.ListPerformersForNetwork(ctx, req.Msg.GetNetworkId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.ListPerformersForNetworkResponse{Performers: performerViewsToProto(views), NextPageToken: next}), nil
}

// ListPerformers implements afterdarkv1connect.BrowseServiceHandler.
func (h *BrowseHandler) ListPerformers(ctx context.Context, req *connect.Request[afterdarkv1.ListPerformersRequest]) (*connect.Response[afterdarkv1.ListPerformersResponse], error) {
	views, next, err := h.svc.ListPerformers(ctx, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.ListPerformersResponse{Performers: performerViewsToProto(views), NextPageToken: next}), nil
}
