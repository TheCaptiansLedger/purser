package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// mediaFileService is the narrow interface MediaFileHandler depends on —
// see personService for the DIP convention this follows.
type mediaFileService interface {
	Create(ctx context.Context, m *domain.MediaFile) (*domain.MediaFile, error)
	Get(ctx context.Context, id string) (*domain.MediaFile, error)
	Update(ctx context.Context, m *domain.MediaFile) (*domain.MediaFile, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, itemID string, pageSize int, pageToken string) ([]*domain.MediaFile, string, error)
}

// MediaFileHandler implements domainv1connect.MediaFileServiceHandler.
type MediaFileHandler struct {
	domainv1connect.UnimplementedMediaFileServiceHandler
	svc    mediaFileService
	logger *slog.Logger
}

// NewMediaFileHandler constructs a MediaFileHandler backed by svc.
func NewMediaFileHandler(svc mediaFileService, logger *slog.Logger) *MediaFileHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MediaFileHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "MediaFileService")}
}

// CreateMediaFile implements domainv1connect.MediaFileServiceHandler.
func (h *MediaFileHandler) CreateMediaFile(ctx context.Context, req *connect.Request[v1.CreateMediaFileRequest]) (*connect.Response[v1.CreateMediaFileResponse], error) {
	m := mediaFileFromProto(req.Msg.GetMediaFile())
	created, err := h.svc.Create(ctx, m)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateMediaFileResponse{MediaFile: mediaFileToProto(created)}), nil
}

// GetMediaFile implements domainv1connect.MediaFileServiceHandler.
func (h *MediaFileHandler) GetMediaFile(ctx context.Context, req *connect.Request[v1.GetMediaFileRequest]) (*connect.Response[v1.GetMediaFileResponse], error) {
	m, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetMediaFileResponse{MediaFile: mediaFileToProto(m)}), nil
}

// UpdateMediaFile implements domainv1connect.MediaFileServiceHandler.
func (h *MediaFileHandler) UpdateMediaFile(ctx context.Context, req *connect.Request[v1.UpdateMediaFileRequest]) (*connect.Response[v1.UpdateMediaFileResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetMediaFile().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyMediaFileFieldMask(existing, req.Msg.GetMediaFile(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateMediaFileResponse{MediaFile: mediaFileToProto(updated)}), nil
}

// DeleteMediaFile implements domainv1connect.MediaFileServiceHandler.
func (h *MediaFileHandler) DeleteMediaFile(ctx context.Context, req *connect.Request[v1.DeleteMediaFileRequest]) (*connect.Response[v1.DeleteMediaFileResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteMediaFileResponse{}), nil
}

// ListMediaFiles implements domainv1connect.MediaFileServiceHandler.
func (h *MediaFileHandler) ListMediaFiles(ctx context.Context, req *connect.Request[v1.ListMediaFilesRequest]) (*connect.Response[v1.ListMediaFilesResponse], error) {
	files, next, err := h.svc.List(ctx, req.Msg.GetItemId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbFiles := make([]*v1.MediaFile, 0, len(files))
	for _, m := range files {
		pbFiles = append(pbFiles, mediaFileToProto(m))
	}
	return connect.NewResponse(&v1.ListMediaFilesResponse{MediaFiles: pbFiles, NextPageToken: next}), nil
}
