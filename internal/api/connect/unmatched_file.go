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
	Resolve(ctx context.Context, id, itemID string, dismiss bool) (*domain.MediaFile, *domain.UnmatchedFile, error)
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

// ResolveUnmatchedFile implements
// pipelinev1connect.UnmatchedFileServiceHandler. It translates the wire
// oneof to plain args — GetItemId()/GetDismiss() already return the zero
// value for the case that wasn't set, so no switch on the oneof is needed
// here; the service rejects the "neither set" shape as a
// *domain.ValidationError, mapped the same way as any other invalid
// argument.
func (h *UnmatchedFileHandler) ResolveUnmatchedFile(ctx context.Context, req *connect.Request[pipelinev1.ResolveUnmatchedFileRequest]) (*connect.Response[pipelinev1.ResolveUnmatchedFileResponse], error) {
	mf, uf, err := h.svc.Resolve(ctx, req.Msg.GetUnmatchedFileId(), req.Msg.GetItemId(), req.Msg.GetDismiss())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	resp := &pipelinev1.ResolveUnmatchedFileResponse{}
	if mf != nil {
		resp.Result = &pipelinev1.ResolveUnmatchedFileResponse_MediaFile{MediaFile: mediaFileToProto(mf)}
	} else {
		resp.Result = &pipelinev1.ResolveUnmatchedFileResponse_UnmatchedFile{UnmatchedFile: unmatchedFileToProto(uf)}
	}
	return connect.NewResponse(resp), nil
}
