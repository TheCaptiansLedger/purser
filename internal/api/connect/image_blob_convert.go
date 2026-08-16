package apiconnect

import (
	"purser/internal/service"

	v1 "purser/gen/go/purser/domain/v1"
)

// blobResultToProto maps service.BlobResult to its wire shape.
func blobResultToProto(r *service.BlobResult) *v1.ImageBlobResponse {
	if r == nil {
		return nil
	}
	return &v1.ImageBlobResponse{
		Key:         r.Key,
		Width:       toInt32(r.Width),
		Height:      toInt32(r.Height),
		ContentType: r.ContentType,
		SizeBytes:   r.SizeBytes,
	}
}
