package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"purser/internal/domain/music"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// musicReleaseService is the narrow interface MusicReleaseHandler depends
// on — see personService for the DIP convention this follows.
type musicReleaseService interface {
	Create(ctx context.Context, r *music.Release) (*music.Release, error)
	Get(ctx context.Context, id string) (*music.Release, error)
	Update(ctx context.Context, r *music.Release) (*music.Release, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) ([]*music.Release, string, error)
}

// MusicReleaseHandler implements
// musicv1connect.MusicReleaseServiceHandler — Music's first
// module-specific handler, added with zero edits to any kernel handler
// file.
type MusicReleaseHandler struct {
	musicv1connect.UnimplementedMusicReleaseServiceHandler
	svc    musicReleaseService
	logger *slog.Logger
}

// NewMusicReleaseHandler constructs a MusicReleaseHandler backed by svc.
func NewMusicReleaseHandler(svc musicReleaseService, logger *slog.Logger) *MusicReleaseHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MusicReleaseHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "MusicReleaseService")}
}

// CreateMusicRelease implements musicv1connect.MusicReleaseServiceHandler.
func (h *MusicReleaseHandler) CreateMusicRelease(ctx context.Context, req *connect.Request[musicv1.CreateMusicReleaseRequest]) (*connect.Response[musicv1.CreateMusicReleaseResponse], error) {
	r := musicReleaseFromProto(req.Msg.GetMusicRelease())
	created, err := h.svc.Create(ctx, r)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.CreateMusicReleaseResponse{MusicRelease: musicReleaseToProto(created)}), nil
}

// GetMusicRelease implements musicv1connect.MusicReleaseServiceHandler.
func (h *MusicReleaseHandler) GetMusicRelease(ctx context.Context, req *connect.Request[musicv1.GetMusicReleaseRequest]) (*connect.Response[musicv1.GetMusicReleaseResponse], error) {
	r, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.GetMusicReleaseResponse{MusicRelease: musicReleaseToProto(r)}), nil
}

// UpdateMusicRelease implements musicv1connect.MusicReleaseServiceHandler.
func (h *MusicReleaseHandler) UpdateMusicRelease(ctx context.Context, req *connect.Request[musicv1.UpdateMusicReleaseRequest]) (*connect.Response[musicv1.UpdateMusicReleaseResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetMusicRelease().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyMusicReleaseFieldMask(existing, req.Msg.GetMusicRelease(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.UpdateMusicReleaseResponse{MusicRelease: musicReleaseToProto(updated)}), nil
}

// DeleteMusicRelease implements musicv1connect.MusicReleaseServiceHandler.
// Cascade is intentionally unused — see DeleteMusicReleaseRequest.cascade's
// doc comment.
func (h *MusicReleaseHandler) DeleteMusicRelease(ctx context.Context, req *connect.Request[musicv1.DeleteMusicReleaseRequest]) (*connect.Response[musicv1.DeleteMusicReleaseResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.DeleteMusicReleaseResponse{}), nil
}

// ListMusicReleases implements musicv1connect.MusicReleaseServiceHandler.
func (h *MusicReleaseHandler) ListMusicReleases(ctx context.Context, req *connect.Request[musicv1.ListMusicReleasesRequest]) (*connect.Response[musicv1.ListMusicReleasesResponse], error) {
	releases, next, err := h.svc.List(ctx, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbReleases := make([]*musicv1.Release, 0, len(releases))
	for _, r := range releases {
		pbReleases = append(pbReleases, musicReleaseToProto(r))
	}
	return connect.NewResponse(&musicv1.ListMusicReleasesResponse{MusicReleases: pbReleases, NextPageToken: next}), nil
}
