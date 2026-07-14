package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// itemService is the narrow interface ItemHandler depends on — see
// personService for the DIP convention this follows.
type itemService interface {
	Create(ctx context.Context, i *domain.Item) (*domain.Item, error)
	Get(ctx context.Context, id string) (*domain.Item, error)
	Update(ctx context.Context, i *domain.Item) (*domain.Item, error)
	List(ctx context.Context, libraryEntryID, contentType, groupID string, pageSize int, pageToken string) ([]*domain.Item, string, error)
}

// ItemHandler implements domainv1connect.ItemServiceHandler.
type ItemHandler struct {
	domainv1connect.UnimplementedItemServiceHandler
	svc         itemService
	deletionSvc bulkDeletionService
	logger      *slog.Logger
}

// NewItemHandler constructs an ItemHandler backed by svc and deletionSvc.
func NewItemHandler(svc itemService, deletionSvc bulkDeletionService, logger *slog.Logger) *ItemHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ItemHandler{svc: svc, deletionSvc: deletionSvc, logger: logger.With("component", "api.connect", "service", "ItemService")}
}

// CreateItem implements domainv1connect.ItemServiceHandler.
func (h *ItemHandler) CreateItem(ctx context.Context, req *connect.Request[v1.CreateItemRequest]) (*connect.Response[v1.CreateItemResponse], error) {
	i := itemFromProto(req.Msg.GetItem())
	created, err := h.svc.Create(ctx, i)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateItemResponse{Item: itemToProto(created)}), nil
}

// GetItem implements domainv1connect.ItemServiceHandler.
func (h *ItemHandler) GetItem(ctx context.Context, req *connect.Request[v1.GetItemRequest]) (*connect.Response[v1.GetItemResponse], error) {
	i, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetItemResponse{Item: itemToProto(i)}), nil
}

// UpdateItem implements domainv1connect.ItemServiceHandler.
func (h *ItemHandler) UpdateItem(ctx context.Context, req *connect.Request[v1.UpdateItemRequest]) (*connect.Response[v1.UpdateItemResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetItem().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyItemFieldMask(existing, req.Msg.GetItem(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateItemResponse{Item: itemToProto(updated)}), nil
}

// DeleteItem implements domainv1connect.ItemServiceHandler.
func (h *ItemHandler) DeleteItem(ctx context.Context, req *connect.Request[v1.DeleteItemRequest]) (*connect.Response[v1.DeleteItemResponse], error) {
	if err := h.deletionSvc.Delete(ctx, req.Msg.GetId(), req.Msg.GetCascade()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteItemResponse{}), nil
}

// GetItemDeletionImpact implements domainv1connect.ItemServiceHandler.
func (h *ItemHandler) GetItemDeletionImpact(ctx context.Context, req *connect.Request[v1.GetItemDeletionImpactRequest]) (*connect.Response[v1.GetItemDeletionImpactResponse], error) {
	impact, err := h.deletionSvc.GetDeletionImpact(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetItemDeletionImpactResponse{Impacts: deletionImpactRowsToProto(impact.Impacts)}), nil
}

// BulkDeleteItems implements domainv1connect.ItemServiceHandler.
func (h *ItemHandler) BulkDeleteItems(ctx context.Context, req *connect.Request[v1.BulkDeleteItemsRequest]) (*connect.Response[v1.BulkDeleteItemsResponse], error) {
	if err := h.deletionSvc.DeleteBatch(ctx, req.Msg.GetIds(), req.Msg.GetCascade()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.BulkDeleteItemsResponse{}), nil
}

// ListItems implements domainv1connect.ItemServiceHandler.
func (h *ItemHandler) ListItems(ctx context.Context, req *connect.Request[v1.ListItemsRequest]) (*connect.Response[v1.ListItemsResponse], error) {
	items, next, err := h.svc.List(ctx, req.Msg.GetLibraryEntryId(), req.Msg.GetContentType(), req.Msg.GetGroupId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbItems := make([]*v1.Item, 0, len(items))
	for _, i := range items {
		pbItems = append(pbItems, itemToProto(i))
	}
	return connect.NewResponse(&v1.ListItemsResponse{Items: pbItems, NextPageToken: next}), nil
}
