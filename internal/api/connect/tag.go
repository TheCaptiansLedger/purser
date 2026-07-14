package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// tagService is the narrow interface TagHandler depends on — see
// personService for the DIP convention this follows.
type tagService interface {
	Create(ctx context.Context, t *domain.Tag) (*domain.Tag, error)
	Get(ctx context.Context, id string) (*domain.Tag, error)
	Update(ctx context.Context, t *domain.Tag) (*domain.Tag, error)
	List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Tag, string, error)
}

// TagHandler implements domainv1connect.TagServiceHandler.
type TagHandler struct {
	domainv1connect.UnimplementedTagServiceHandler
	svc         tagService
	deletionSvc entityDeletionService
	logger      *slog.Logger
}

// NewTagHandler constructs a TagHandler backed by svc and deletionSvc.
func NewTagHandler(svc tagService, deletionSvc entityDeletionService, logger *slog.Logger) *TagHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TagHandler{svc: svc, deletionSvc: deletionSvc, logger: logger.With("component", "api.connect", "service", "TagService")}
}

// CreateTag implements domainv1connect.TagServiceHandler.
func (h *TagHandler) CreateTag(ctx context.Context, req *connect.Request[v1.CreateTagRequest]) (*connect.Response[v1.CreateTagResponse], error) {
	t := tagFromProto(req.Msg.GetTag())
	created, err := h.svc.Create(ctx, t)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateTagResponse{Tag: tagToProto(created)}), nil
}

// GetTag implements domainv1connect.TagServiceHandler.
func (h *TagHandler) GetTag(ctx context.Context, req *connect.Request[v1.GetTagRequest]) (*connect.Response[v1.GetTagResponse], error) {
	t, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetTagResponse{Tag: tagToProto(t)}), nil
}

// UpdateTag implements domainv1connect.TagServiceHandler.
func (h *TagHandler) UpdateTag(ctx context.Context, req *connect.Request[v1.UpdateTagRequest]) (*connect.Response[v1.UpdateTagResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetTag().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyTagFieldMask(existing, req.Msg.GetTag(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateTagResponse{Tag: tagToProto(updated)}), nil
}

// DeleteTag implements domainv1connect.TagServiceHandler.
func (h *TagHandler) DeleteTag(ctx context.Context, req *connect.Request[v1.DeleteTagRequest]) (*connect.Response[v1.DeleteTagResponse], error) {
	if err := h.deletionSvc.Delete(ctx, req.Msg.GetId(), req.Msg.GetCascade()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteTagResponse{}), nil
}

// GetTagDeletionImpact implements domainv1connect.TagServiceHandler.
func (h *TagHandler) GetTagDeletionImpact(ctx context.Context, req *connect.Request[v1.GetTagDeletionImpactRequest]) (*connect.Response[v1.GetTagDeletionImpactResponse], error) {
	impact, err := h.deletionSvc.GetDeletionImpact(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetTagDeletionImpactResponse{Impacts: deletionImpactRowsToProto(impact.Impacts)}), nil
}

// ListTags implements domainv1connect.TagServiceHandler.
func (h *TagHandler) ListTags(ctx context.Context, req *connect.Request[v1.ListTagsRequest]) (*connect.Response[v1.ListTagsResponse], error) {
	tags, next, err := h.svc.List(ctx, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbTags := make([]*v1.Tag, 0, len(tags))
	for _, t := range tags {
		pbTags = append(pbTags, tagToProto(t))
	}
	return connect.NewResponse(&v1.ListTagsResponse{Tags: pbTags, NextPageToken: next}), nil
}
