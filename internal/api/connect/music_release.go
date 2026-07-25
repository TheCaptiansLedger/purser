package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"purser/internal/domain"
	"purser/internal/domain/music"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// musicReleaseService is the narrow interface MusicReleaseHandler depends
// on — see personService for the DIP convention this follows.
type musicReleaseService interface {
	Create(ctx context.Context, r *music.Release) (*music.Release, error)
	Get(ctx context.Context, id string) (*music.Release, error)
	GetByMBID(ctx context.Context, mbid string) (*music.Release, error)
	GetByBarcode(ctx context.Context, barcode string) (*music.Release, error)
	Update(ctx context.Context, r *music.Release) (*music.Release, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) ([]*music.Release, string, error)
	ListByGroup(ctx context.Context, groupID string, pageSize int, pageToken string) ([]*music.Release, string, error)
	ListByEntry(ctx context.Context, libraryEntryID string, pageSize int, pageToken string) ([]*music.Release, string, error)
	ListTracksByRelease(ctx context.Context, releaseID string, pageSize int, pageToken string) ([]*domain.Item, string, error)
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

// GetMusicReleaseByMBID implements
// musicv1connect.MusicReleaseServiceHandler as an indexed point lookup.
func (h *MusicReleaseHandler) GetMusicReleaseByMBID(ctx context.Context, req *connect.Request[musicv1.GetMusicReleaseByMBIDRequest]) (*connect.Response[musicv1.GetMusicReleaseByMBIDResponse], error) {
	r, err := h.svc.GetByMBID(ctx, req.Msg.GetMbid())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.GetMusicReleaseByMBIDResponse{MusicRelease: musicReleaseToProto(r)}), nil
}

// GetMusicReleaseByBarcode implements
// musicv1connect.MusicReleaseServiceHandler as an indexed point lookup.
func (h *MusicReleaseHandler) GetMusicReleaseByBarcode(ctx context.Context, req *connect.Request[musicv1.GetMusicReleaseByBarcodeRequest]) (*connect.Response[musicv1.GetMusicReleaseByBarcodeResponse], error) {
	r, err := h.svc.GetByBarcode(ctx, req.Msg.GetBarcode())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.GetMusicReleaseByBarcodeResponse{MusicRelease: musicReleaseToProto(r)}), nil
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
// group_id and library_entry_id are independent, optional filters; if both
// are set, group_id takes precedence — see ListMusicReleasesRequest's doc
// comment.
func (h *MusicReleaseHandler) ListMusicReleases(ctx context.Context, req *connect.Request[musicv1.ListMusicReleasesRequest]) (*connect.Response[musicv1.ListMusicReleasesResponse], error) {
	pageSize, pageToken := int(req.Msg.GetPageSize()), req.Msg.GetPageToken()

	var releases []*music.Release
	var next string
	var err error
	switch {
	case req.Msg.GetGroupId() != "":
		releases, next, err = h.svc.ListByGroup(ctx, req.Msg.GetGroupId(), pageSize, pageToken)
	case req.Msg.GetLibraryEntryId() != "":
		releases, next, err = h.svc.ListByEntry(ctx, req.Msg.GetLibraryEntryId(), pageSize, pageToken)
	default:
		releases, next, err = h.svc.List(ctx, pageSize, pageToken)
	}
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbReleases := make([]*musicv1.Release, 0, len(releases))
	for _, r := range releases {
		pbReleases = append(pbReleases, musicReleaseToProto(r))
	}
	return connect.NewResponse(&musicv1.ListMusicReleasesResponse{MusicReleases: pbReleases, NextPageToken: next}), nil
}

// ListMusicReleaseTracks implements
// musicv1connect.MusicReleaseServiceHandler. Returns purser.domain.v1.Item
// directly via the shared itemsToProto helper (also used by BrowseHandler's
// scene-listing RPCs) — a track is an ordinary kernel Item, not a
// Music-specific wire type.
func (h *MusicReleaseHandler) ListMusicReleaseTracks(ctx context.Context, req *connect.Request[musicv1.ListMusicReleaseTracksRequest]) (*connect.Response[musicv1.ListMusicReleaseTracksResponse], error) {
	tracks, next, err := h.svc.ListTracksByRelease(ctx, req.Msg.GetReleaseId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&musicv1.ListMusicReleaseTracksResponse{Tracks: itemsToProto(tracks), NextPageToken: next}), nil
}
