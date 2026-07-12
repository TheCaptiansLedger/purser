package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// itemPersonService is the narrow interface ItemPersonHandler depends on
// — see personService for the DIP convention this follows.
type itemPersonService interface {
	Create(ctx context.Context, ip *domain.ItemPerson) (*domain.ItemPerson, error)
	Get(ctx context.Context, itemID, personID, role string) (*domain.ItemPerson, error)
	Update(ctx context.Context, ip *domain.ItemPerson) (*domain.ItemPerson, error)
	Delete(ctx context.Context, itemID, personID, role string) error
	List(ctx context.Context, itemID, personID string, pageSize int, pageToken string) ([]*domain.ItemPerson, string, error)
}

// ItemPersonHandler implements domainv1connect.ItemPersonServiceHandler.
type ItemPersonHandler struct {
	domainv1connect.UnimplementedItemPersonServiceHandler
	svc    itemPersonService
	logger *slog.Logger
}

// NewItemPersonHandler constructs an ItemPersonHandler backed by svc.
func NewItemPersonHandler(svc itemPersonService, logger *slog.Logger) *ItemPersonHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ItemPersonHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "ItemPersonService")}
}

// CreateItemPerson implements domainv1connect.ItemPersonServiceHandler.
func (h *ItemPersonHandler) CreateItemPerson(ctx context.Context, req *connect.Request[v1.CreateItemPersonRequest]) (*connect.Response[v1.CreateItemPersonResponse], error) {
	ip := itemPersonFromProto(req.Msg.GetItemPerson())
	created, err := h.svc.Create(ctx, ip)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateItemPersonResponse{ItemPerson: itemPersonToProto(created)}), nil
}

// GetItemPerson implements domainv1connect.ItemPersonServiceHandler.
func (h *ItemPersonHandler) GetItemPerson(ctx context.Context, req *connect.Request[v1.GetItemPersonRequest]) (*connect.Response[v1.GetItemPersonResponse], error) {
	ip, err := h.svc.Get(ctx, req.Msg.GetItemId(), req.Msg.GetPersonId(), req.Msg.GetRole())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetItemPersonResponse{ItemPerson: itemPersonToProto(ip)}), nil
}

// UpdateItemPerson implements domainv1connect.ItemPersonServiceHandler.
func (h *ItemPersonHandler) UpdateItemPerson(ctx context.Context, req *connect.Request[v1.UpdateItemPersonRequest]) (*connect.Response[v1.UpdateItemPersonResponse], error) {
	incoming := req.Msg.GetItemPerson()
	existing, err := h.svc.Get(ctx, incoming.GetItemId(), incoming.GetPersonId(), incoming.GetRole())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyItemPersonFieldMask(existing, incoming, req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateItemPersonResponse{ItemPerson: itemPersonToProto(updated)}), nil
}

// DeleteItemPerson implements domainv1connect.ItemPersonServiceHandler.
func (h *ItemPersonHandler) DeleteItemPerson(ctx context.Context, req *connect.Request[v1.DeleteItemPersonRequest]) (*connect.Response[v1.DeleteItemPersonResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetItemId(), req.Msg.GetPersonId(), req.Msg.GetRole()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteItemPersonResponse{}), nil
}

// ListItemPeople implements domainv1connect.ItemPersonServiceHandler.
func (h *ItemPersonHandler) ListItemPeople(ctx context.Context, req *connect.Request[v1.ListItemPeopleRequest]) (*connect.Response[v1.ListItemPeopleResponse], error) {
	rows, next, err := h.svc.List(ctx, req.Msg.GetItemId(), req.Msg.GetPersonId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbRows := make([]*v1.ItemPerson, 0, len(rows))
	for _, ip := range rows {
		pbRows = append(pbRows, itemPersonToProto(ip))
	}
	return connect.NewResponse(&v1.ListItemPeopleResponse{ItemPeople: pbRows, NextPageToken: next}), nil
}
