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
}

// MusicReleaseHandler implements
// musicv1connect.MusicReleaseServiceHandler — Music's first
// module-specific handler, added with zero edits to any kernel handler
// file. Only CreateMusicRelease/GetMusicRelease are implemented in this
// walking-skeleton pass.
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
