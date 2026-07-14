package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// tagAssignmentService is the narrow interface TagAssignmentHandler depends
// on — see personService for the DIP convention this follows. There is no
// Update — nothing about a TagAssignment is mutable.
type tagAssignmentService interface {
	Create(ctx context.Context, ta *domain.TagAssignment) (*domain.TagAssignment, error)
	Get(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) (*domain.TagAssignment, error)
	Delete(ctx context.Context, tagID string, entityType domain.EntityType, entityID string) error
	List(ctx context.Context, tagID string, entityType domain.EntityType, entityID string, pageSize int, pageToken string) ([]*domain.TagAssignment, string, error)
	BulkCreateTagAssignments(ctx context.Context, tagID string, entityType domain.EntityType, entityIDs []string) ([]*domain.TagAssignment, error)
}

// TagAssignmentHandler implements domainv1connect.TagAssignmentServiceHandler.
type TagAssignmentHandler struct {
	domainv1connect.UnimplementedTagAssignmentServiceHandler
	svc    tagAssignmentService
	logger *slog.Logger
}

// NewTagAssignmentHandler constructs a TagAssignmentHandler backed by svc.
func NewTagAssignmentHandler(svc tagAssignmentService, logger *slog.Logger) *TagAssignmentHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TagAssignmentHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "TagAssignmentService")}
}

// CreateTagAssignment implements domainv1connect.TagAssignmentServiceHandler.
func (h *TagAssignmentHandler) CreateTagAssignment(ctx context.Context, req *connect.Request[v1.CreateTagAssignmentRequest]) (*connect.Response[v1.CreateTagAssignmentResponse], error) {
	ta := tagAssignmentFromProto(req.Msg.GetTagAssignment())
	created, err := h.svc.Create(ctx, ta)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateTagAssignmentResponse{TagAssignment: tagAssignmentToProto(created)}), nil
}

// GetTagAssignment implements domainv1connect.TagAssignmentServiceHandler.
func (h *TagAssignmentHandler) GetTagAssignment(ctx context.Context, req *connect.Request[v1.GetTagAssignmentRequest]) (*connect.Response[v1.GetTagAssignmentResponse], error) {
	ta, err := h.svc.Get(ctx, req.Msg.GetTagId(), entityTypeFromProto(req.Msg.GetEntityType()), req.Msg.GetEntityId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetTagAssignmentResponse{TagAssignment: tagAssignmentToProto(ta)}), nil
}

// DeleteTagAssignment implements domainv1connect.TagAssignmentServiceHandler.
func (h *TagAssignmentHandler) DeleteTagAssignment(ctx context.Context, req *connect.Request[v1.DeleteTagAssignmentRequest]) (*connect.Response[v1.DeleteTagAssignmentResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetTagId(), entityTypeFromProto(req.Msg.GetEntityType()), req.Msg.GetEntityId()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteTagAssignmentResponse{}), nil
}

// BulkCreateTagAssignments implements domainv1connect.TagAssignmentServiceHandler.
func (h *TagAssignmentHandler) BulkCreateTagAssignments(ctx context.Context, req *connect.Request[v1.BulkCreateTagAssignmentsRequest]) (*connect.Response[v1.BulkCreateTagAssignmentsResponse], error) {
	created, err := h.svc.BulkCreateTagAssignments(ctx, req.Msg.GetTagId(), entityTypeFromProto(req.Msg.GetEntityType()), req.Msg.GetEntityIds())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbRows := make([]*v1.TagAssignment, 0, len(created))
	for _, ta := range created {
		pbRows = append(pbRows, tagAssignmentToProto(ta))
	}
	return connect.NewResponse(&v1.BulkCreateTagAssignmentsResponse{TagAssignments: pbRows}), nil
}

// ListTagAssignments implements domainv1connect.TagAssignmentServiceHandler.
func (h *TagAssignmentHandler) ListTagAssignments(ctx context.Context, req *connect.Request[v1.ListTagAssignmentsRequest]) (*connect.Response[v1.ListTagAssignmentsResponse], error) {
	rows, next, err := h.svc.List(ctx, req.Msg.GetTagId(), entityTypeFromProto(req.Msg.GetEntityType()), req.Msg.GetEntityId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbRows := make([]*v1.TagAssignment, 0, len(rows))
	for _, ta := range rows {
		pbRows = append(pbRows, tagAssignmentToProto(ta))
	}
	return connect.NewResponse(&v1.ListTagAssignmentsResponse{TagAssignments: pbRows, NextPageToken: next}), nil
}
