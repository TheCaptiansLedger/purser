package apiconnect

import (
	"context"
	"log/slog"
	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
	"purser/gen/go/purser/acquisition/v1/acquisitionv1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"
)

// downloadService is the narrow interface DownloadHandler depends on — see
// personService for the DIP convention this follows.
type downloadService interface {
	SubmitDownload(ctx context.Context, req ports.AddDownloadRequest) (string, error)
	GetDownloadStatus(ctx context.Context, protocol ports.Protocol, externalID string) (ports.DownloadStatus, error)
	RemoveDownload(ctx context.Context, protocol ports.Protocol, externalID string, deleteFiles bool) error
}

// DownloadHandler implements acquisitionv1connect.DownloadServiceHandler —
// see proto/purser/acquisition/v1/download.proto's own doc comment for why
// this exists.
type DownloadHandler struct {
	acquisitionv1connect.UnimplementedDownloadServiceHandler
	svc    downloadService
	logger *slog.Logger
}

// NewDownloadHandler constructs a DownloadHandler backed by svc.
func NewDownloadHandler(svc downloadService, logger *slog.Logger) *DownloadHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DownloadHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "DownloadService")}
}

// SubmitDownload implements acquisitionv1connect.DownloadServiceHandler.
func (h *DownloadHandler) SubmitDownload(ctx context.Context, req *connect.Request[acquisitionv1.SubmitDownloadRequest]) (*connect.Response[acquisitionv1.SubmitDownloadResponse], error) {
	externalID, err := h.svc.SubmitDownload(ctx, ports.AddDownloadRequest{
		Protocol:    protoToProtocol(req.Msg.GetProtocol()),
		DownloadURL: req.Msg.GetDownloadUrl(),
		Title:       req.Msg.GetTitle(),
		Category:    req.Msg.GetCategory(),
	})
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&acquisitionv1.SubmitDownloadResponse{ExternalId: externalID}), nil
}

// GetDownloadStatus implements acquisitionv1connect.DownloadServiceHandler.
func (h *DownloadHandler) GetDownloadStatus(ctx context.Context, req *connect.Request[acquisitionv1.GetDownloadStatusRequest]) (*connect.Response[acquisitionv1.GetDownloadStatusResponse], error) {
	status, err := h.svc.GetDownloadStatus(ctx, protoToProtocol(req.Msg.GetProtocol()), req.Msg.GetExternalId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&acquisitionv1.GetDownloadStatusResponse{Status: downloadStatusToProto(status)}), nil
}

// RemoveDownload implements acquisitionv1connect.DownloadServiceHandler.
func (h *DownloadHandler) RemoveDownload(ctx context.Context, req *connect.Request[acquisitionv1.RemoveDownloadRequest]) (*connect.Response[acquisitionv1.RemoveDownloadResponse], error) {
	err := h.svc.RemoveDownload(ctx, protoToProtocol(req.Msg.GetProtocol()), req.Msg.GetExternalId(), req.Msg.GetDeleteFiles())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&acquisitionv1.RemoveDownloadResponse{}), nil
}
