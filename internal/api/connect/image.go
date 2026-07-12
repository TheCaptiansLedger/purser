package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// imageService is the narrow interface ImageHandler depends on — see
// personService for the DIP convention this follows.
type imageService interface {
	Create(ctx context.Context, img *domain.Image) (*domain.Image, error)
	Get(ctx context.Context, id string) (*domain.Image, error)
	Update(ctx context.Context, img *domain.Image) (*domain.Image, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, ownerType, ownerID string, pageSize int, pageToken string) ([]*domain.Image, string, error)
}

// ImageHandler implements domainv1connect.ImageServiceHandler.
type ImageHandler struct {
	domainv1connect.UnimplementedImageServiceHandler
	svc    imageService
	logger *slog.Logger
}

// NewImageHandler constructs an ImageHandler backed by svc.
func NewImageHandler(svc imageService, logger *slog.Logger) *ImageHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ImageHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "ImageService")}
}

// CreateImage implements domainv1connect.ImageServiceHandler.
func (h *ImageHandler) CreateImage(ctx context.Context, req *connect.Request[v1.CreateImageRequest]) (*connect.Response[v1.CreateImageResponse], error) {
	img := imageFromProto(req.Msg.GetImage())
	created, err := h.svc.Create(ctx, img)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateImageResponse{Image: imageToProto(created)}), nil
}

// GetImage implements domainv1connect.ImageServiceHandler.
func (h *ImageHandler) GetImage(ctx context.Context, req *connect.Request[v1.GetImageRequest]) (*connect.Response[v1.GetImageResponse], error) {
	img, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetImageResponse{Image: imageToProto(img)}), nil
}

// UpdateImage implements domainv1connect.ImageServiceHandler.
func (h *ImageHandler) UpdateImage(ctx context.Context, req *connect.Request[v1.UpdateImageRequest]) (*connect.Response[v1.UpdateImageResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetImage().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyImageFieldMask(existing, req.Msg.GetImage(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateImageResponse{Image: imageToProto(updated)}), nil
}

// DeleteImage implements domainv1connect.ImageServiceHandler.
func (h *ImageHandler) DeleteImage(ctx context.Context, req *connect.Request[v1.DeleteImageRequest]) (*connect.Response[v1.DeleteImageResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteImageResponse{}), nil
}

// ListImages implements domainv1connect.ImageServiceHandler.
func (h *ImageHandler) ListImages(ctx context.Context, req *connect.Request[v1.ListImagesRequest]) (*connect.Response[v1.ListImagesResponse], error) {
	images, next, err := h.svc.List(ctx, req.Msg.GetOwnerType(), req.Msg.GetOwnerId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbImages := make([]*v1.Image, 0, len(images))
	for _, img := range images {
		pbImages = append(pbImages, imageToProto(img))
	}
	return connect.NewResponse(&v1.ListImagesResponse{Images: pbImages, NextPageToken: next}), nil
}
