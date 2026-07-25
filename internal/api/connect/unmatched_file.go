package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/pipeline/v1/pipelinev1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	pipelinev1 "purser/gen/go/purser/pipeline/v1"
)

// unmatchedFileService is the narrow interface UnmatchedFileHandler
// depends on — see personService for the DIP convention this follows.
type unmatchedFileService interface {
	Get(ctx context.Context, id string) (*domain.UnmatchedFile, error)
	List(ctx context.Context, status domain.UnmatchedFileStatus, pageSize int, pageToken string) ([]*domain.UnmatchedFile, string, error)
}

// UnmatchedFileHandler implements
// pipelinev1connect.UnmatchedFileServiceHandler.
type UnmatchedFileHandler struct {
	pipelinev1connect.UnimplementedUnmatchedFileServiceHandler
	svc    unmatchedFileService
	logger *slog.Logger
}

// NewUnmatchedFileHandler constructs an UnmatchedFileHandler backed by
// svc.
func NewUnmatchedFileHandler(svc unmatchedFileService, logger *slog.Logger) *UnmatchedFileHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &UnmatchedFileHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "UnmatchedFileService")}
}

// GetUnmatchedFile implements
// pipelinev1connect.UnmatchedFileServiceHandler.
func (h *UnmatchedFileHandler) GetUnmatchedFile(ctx context.Context, req *connect.Request[pipelinev1.GetUnmatchedFileRequest]) (*connect.Response[pipelinev1.GetUnmatchedFileResponse], error) {
	u, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&pipelinev1.GetUnmatchedFileResponse{UnmatchedFile: unmatchedFileToProto(u)}), nil
}

// ListUnmatchedFiles implements
// pipelinev1connect.UnmatchedFileServiceHandler.
func (h *UnmatchedFileHandler) ListUnmatchedFiles(ctx context.Context, req *connect.Request[pipelinev1.ListUnmatchedFilesRequest]) (*connect.Response[pipelinev1.ListUnmatchedFilesResponse], error) {
	files, next, err := h.svc.List(ctx, protoToUnmatchedFileStatus(req.Msg.GetStatus()), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbFiles := make([]*pipelinev1.UnmatchedFile, 0, len(files))
	for _, u := range files {
		pbFiles = append(pbFiles, unmatchedFileToProto(u))
	}
	return connect.NewResponse(&pipelinev1.ListUnmatchedFilesResponse{UnmatchedFiles: pbFiles, NextPageToken: next}), nil
}
