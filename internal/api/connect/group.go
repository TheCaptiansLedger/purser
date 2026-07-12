package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// groupService is the narrow interface GroupHandler depends on — see
// personService for the DIP convention this follows.
type groupService interface {
	Create(ctx context.Context, g *domain.Group) (*domain.Group, error)
	Get(ctx context.Context, id string) (*domain.Group, error)
	Update(ctx context.Context, g *domain.Group) (*domain.Group, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Group, string, error)
}

// GroupHandler implements domainv1connect.GroupServiceHandler.
type GroupHandler struct {
	domainv1connect.UnimplementedGroupServiceHandler
	svc    groupService
	logger *slog.Logger
}

// NewGroupHandler constructs a GroupHandler backed by svc.
func NewGroupHandler(svc groupService, logger *slog.Logger) *GroupHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &GroupHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "GroupService")}
}

// CreateGroup implements domainv1connect.GroupServiceHandler.
func (h *GroupHandler) CreateGroup(ctx context.Context, req *connect.Request[v1.CreateGroupRequest]) (*connect.Response[v1.CreateGroupResponse], error) {
	g := groupFromProto(req.Msg.GetGroup())
	created, err := h.svc.Create(ctx, g)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateGroupResponse{Group: groupToProto(created)}), nil
}

// GetGroup implements domainv1connect.GroupServiceHandler.
func (h *GroupHandler) GetGroup(ctx context.Context, req *connect.Request[v1.GetGroupRequest]) (*connect.Response[v1.GetGroupResponse], error) {
	g, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetGroupResponse{Group: groupToProto(g)}), nil
}

// UpdateGroup implements domainv1connect.GroupServiceHandler.
func (h *GroupHandler) UpdateGroup(ctx context.Context, req *connect.Request[v1.UpdateGroupRequest]) (*connect.Response[v1.UpdateGroupResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetGroup().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyGroupFieldMask(existing, req.Msg.GetGroup(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateGroupResponse{Group: groupToProto(updated)}), nil
}

// DeleteGroup implements domainv1connect.GroupServiceHandler.
func (h *GroupHandler) DeleteGroup(ctx context.Context, req *connect.Request[v1.DeleteGroupRequest]) (*connect.Response[v1.DeleteGroupResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteGroupResponse{}), nil
}

// ListGroups implements domainv1connect.GroupServiceHandler.
func (h *GroupHandler) ListGroups(ctx context.Context, req *connect.Request[v1.ListGroupsRequest]) (*connect.Response[v1.ListGroupsResponse], error) {
	groups, next, err := h.svc.List(ctx, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbGroups := make([]*v1.Group, 0, len(groups))
	for _, g := range groups {
		pbGroups = append(pbGroups, groupToProto(g))
	}
	return connect.NewResponse(&v1.ListGroupsResponse{Groups: pbGroups, NextPageToken: next}), nil
}
