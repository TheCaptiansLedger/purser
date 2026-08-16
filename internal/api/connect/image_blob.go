package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/service"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// imageBlobService is the narrow interface ImageBlobHandler depends on —
// see personService for the DIP convention this follows.
type imageBlobService interface {
	CacheRemoteImage(ctx context.Context, url string) (*service.BlobResult, error)
	UploadImage(ctx context.Context, data []byte) (*service.BlobResult, error)
}

// ImageBlobHandler implements domainv1connect.ImageBlobServiceHandler —
// see proto/purser/domain/v1/image_blob.proto's own doc comment, and
// docs/adr/0013-image-blob-storage.md / docs/technical/image-caching-and-serving.md
// for why this exists.
type ImageBlobHandler struct {
	domainv1connect.UnimplementedImageBlobServiceHandler
	svc    imageBlobService
	logger *slog.Logger
}

// NewImageBlobHandler constructs an ImageBlobHandler backed by svc.
func NewImageBlobHandler(svc imageBlobService, logger *slog.Logger) *ImageBlobHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ImageBlobHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "ImageBlobService")}
}

// CacheRemoteImage implements domainv1connect.ImageBlobServiceHandler.
func (h *ImageBlobHandler) CacheRemoteImage(ctx context.Context, req *connect.Request[v1.CacheRemoteImageRequest]) (*connect.Response[v1.CacheRemoteImageResponse], error) {
	result, err := h.svc.CacheRemoteImage(ctx, req.Msg.GetUrl())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CacheRemoteImageResponse{Blob: blobResultToProto(result)}), nil
}

// UploadImage implements domainv1connect.ImageBlobServiceHandler.
func (h *ImageBlobHandler) UploadImage(ctx context.Context, req *connect.Request[v1.UploadImageRequest]) (*connect.Response[v1.UploadImageResponse], error) {
	result, err := h.svc.UploadImage(ctx, req.Msg.GetData())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UploadImageResponse{Blob: blobResultToProto(result)}), nil
}
