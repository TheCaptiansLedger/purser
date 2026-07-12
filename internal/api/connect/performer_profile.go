package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	"purser/internal/domain/afterdark"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// performerProfileService is the narrow interface PerformerProfileHandler
// depends on — see personService for the DIP convention this follows.
type performerProfileService interface {
	Create(ctx context.Context, p *afterdark.PerformerProfile) (*afterdark.PerformerProfile, error)
	Get(ctx context.Context, personID string) (*afterdark.PerformerProfile, error)
	Update(ctx context.Context, p *afterdark.PerformerProfile) (*afterdark.PerformerProfile, error)
	Delete(ctx context.Context, personID string) error
	List(ctx context.Context, pageSize int, pageToken string) ([]*afterdark.PerformerProfile, string, error)
}

// PerformerProfileHandler implements
// afterdarkv1connect.PerformerProfileServiceHandler — AfterDark's only
// module-specific handler, added with zero edits to any kernel handler
// file.
type PerformerProfileHandler struct {
	afterdarkv1connect.UnimplementedPerformerProfileServiceHandler
	svc    performerProfileService
	logger *slog.Logger
}

// NewPerformerProfileHandler constructs a PerformerProfileHandler backed
// by svc.
func NewPerformerProfileHandler(svc performerProfileService, logger *slog.Logger) *PerformerProfileHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PerformerProfileHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "PerformerProfileService")}
}

// CreatePerformerProfile implements afterdarkv1connect.PerformerProfileServiceHandler.
func (h *PerformerProfileHandler) CreatePerformerProfile(ctx context.Context, req *connect.Request[afterdarkv1.CreatePerformerProfileRequest]) (*connect.Response[afterdarkv1.CreatePerformerProfileResponse], error) {
	p := performerProfileFromProto(req.Msg.GetPerformerProfile())
	created, err := h.svc.Create(ctx, p)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.CreatePerformerProfileResponse{PerformerProfile: performerProfileToProto(created)}), nil
}

// GetPerformerProfile implements afterdarkv1connect.PerformerProfileServiceHandler.
func (h *PerformerProfileHandler) GetPerformerProfile(ctx context.Context, req *connect.Request[afterdarkv1.GetPerformerProfileRequest]) (*connect.Response[afterdarkv1.GetPerformerProfileResponse], error) {
	p, err := h.svc.Get(ctx, req.Msg.GetPersonId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.GetPerformerProfileResponse{PerformerProfile: performerProfileToProto(p)}), nil
}

// UpdatePerformerProfile implements afterdarkv1connect.PerformerProfileServiceHandler.
func (h *PerformerProfileHandler) UpdatePerformerProfile(ctx context.Context, req *connect.Request[afterdarkv1.UpdatePerformerProfileRequest]) (*connect.Response[afterdarkv1.UpdatePerformerProfileResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetPerformerProfile().GetPersonId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyPerformerProfileFieldMask(existing, req.Msg.GetPerformerProfile(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.UpdatePerformerProfileResponse{PerformerProfile: performerProfileToProto(updated)}), nil
}

// DeletePerformerProfile implements afterdarkv1connect.PerformerProfileServiceHandler.
func (h *PerformerProfileHandler) DeletePerformerProfile(ctx context.Context, req *connect.Request[afterdarkv1.DeletePerformerProfileRequest]) (*connect.Response[afterdarkv1.DeletePerformerProfileResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetPersonId()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&afterdarkv1.DeletePerformerProfileResponse{}), nil
}

// ListPerformerProfiles implements afterdarkv1connect.PerformerProfileServiceHandler.
func (h *PerformerProfileHandler) ListPerformerProfiles(ctx context.Context, req *connect.Request[afterdarkv1.ListPerformerProfilesRequest]) (*connect.Response[afterdarkv1.ListPerformerProfilesResponse], error) {
	profiles, next, err := h.svc.List(ctx, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbProfiles := make([]*afterdarkv1.PerformerProfile, 0, len(profiles))
	for _, p := range profiles {
		pbProfiles = append(pbProfiles, performerProfileToProto(p))
	}
	return connect.NewResponse(&afterdarkv1.ListPerformerProfilesResponse{PerformerProfiles: pbProfiles, NextPageToken: next}), nil
}
