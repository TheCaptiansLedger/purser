package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// externalIDService is the narrow interface ExternalIDHandler depends on
// — see personService for the DIP convention this follows.
type externalIDService interface {
	Create(ctx context.Context, e *domain.ExternalID) (*domain.ExternalID, error)
	Get(ctx context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error)
	GetByValue(ctx context.Context, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, error)
	Update(ctx context.Context, e *domain.ExternalID) (*domain.ExternalID, error)
	Delete(ctx context.Context, entityType domain.EntityType, entityID, source string) error
	List(ctx context.Context, entityType domain.EntityType, entityID string, pageSize int, pageToken string) ([]*domain.ExternalID, string, error)
}

// ExternalIDHandler implements domainv1connect.ExternalIDServiceHandler.
type ExternalIDHandler struct {
	domainv1connect.UnimplementedExternalIDServiceHandler
	svc    externalIDService
	logger *slog.Logger
}

// NewExternalIDHandler constructs an ExternalIDHandler backed by svc.
func NewExternalIDHandler(svc externalIDService, logger *slog.Logger) *ExternalIDHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ExternalIDHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "ExternalIDService")}
}

// CreateExternalID implements domainv1connect.ExternalIDServiceHandler.
func (h *ExternalIDHandler) CreateExternalID(ctx context.Context, req *connect.Request[v1.CreateExternalIDRequest]) (*connect.Response[v1.CreateExternalIDResponse], error) {
	e := externalIDFromProto(req.Msg.GetExternalId())
	created, err := h.svc.Create(ctx, e)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateExternalIDResponse{ExternalId: externalIDToProto(created)}), nil
}

// GetExternalID implements domainv1connect.ExternalIDServiceHandler.
func (h *ExternalIDHandler) GetExternalID(ctx context.Context, req *connect.Request[v1.GetExternalIDRequest]) (*connect.Response[v1.GetExternalIDResponse], error) {
	e, err := h.svc.Get(ctx, entityTypeFromProto(req.Msg.GetEntityType()), req.Msg.GetEntityId(), req.Msg.GetSource())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetExternalIDResponse{ExternalId: externalIDToProto(e)}), nil
}

// GetExternalIDByValue implements domainv1connect.ExternalIDServiceHandler.
func (h *ExternalIDHandler) GetExternalIDByValue(ctx context.Context, req *connect.Request[v1.GetExternalIDByValueRequest]) (*connect.Response[v1.GetExternalIDByValueResponse], error) {
	e, err := h.svc.GetByValue(ctx, entityTypeFromProto(req.Msg.GetEntityType()), domain.ExternalIDSource(req.Msg.GetSource()), req.Msg.GetValue())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetExternalIDByValueResponse{ExternalId: externalIDToProto(e)}), nil
}

// UpdateExternalID implements domainv1connect.ExternalIDServiceHandler.
// Only Value is mutable — entity_type/entity_id/source identify the row
// and are pinned to the existing record, not whatever the caller sent.
func (h *ExternalIDHandler) UpdateExternalID(ctx context.Context, req *connect.Request[v1.UpdateExternalIDRequest]) (*connect.Response[v1.UpdateExternalIDResponse], error) {
	incoming := req.Msg.GetExternalId()
	existing, err := h.svc.Get(ctx, entityTypeFromProto(incoming.GetEntityType()), incoming.GetEntityId(), incoming.GetSource())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := *existing
	merged.Value = incoming.GetValue()

	updated, err := h.svc.Update(ctx, &merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateExternalIDResponse{ExternalId: externalIDToProto(updated)}), nil
}

// DeleteExternalID implements domainv1connect.ExternalIDServiceHandler.
func (h *ExternalIDHandler) DeleteExternalID(ctx context.Context, req *connect.Request[v1.DeleteExternalIDRequest]) (*connect.Response[v1.DeleteExternalIDResponse], error) {
	if err := h.svc.Delete(ctx, entityTypeFromProto(req.Msg.GetEntityType()), req.Msg.GetEntityId(), req.Msg.GetSource()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteExternalIDResponse{}), nil
}

// ListExternalIDs implements domainv1connect.ExternalIDServiceHandler.
func (h *ExternalIDHandler) ListExternalIDs(ctx context.Context, req *connect.Request[v1.ListExternalIDsRequest]) (*connect.Response[v1.ListExternalIDsResponse], error) {
	rows, next, err := h.svc.List(ctx, entityTypeFromProto(req.Msg.GetEntityType()), req.Msg.GetEntityId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbRows := make([]*v1.ExternalID, 0, len(rows))
	for _, e := range rows {
		pbRows = append(pbRows, externalIDToProto(e))
	}
	return connect.NewResponse(&v1.ListExternalIDsResponse{ExternalIds: pbRows, NextPageToken: next}), nil
}
