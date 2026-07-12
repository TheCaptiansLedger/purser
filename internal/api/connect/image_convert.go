package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func imageToProto(img *domain.Image) *v1.Image {
	if img == nil {
		return nil
	}
	return &v1.Image{
		Id:        img.ID,
		OwnerType: img.OwnerType,
		OwnerId:   img.OwnerID,
		ImageType: string(img.ImageType),
		Url:       img.URL,
		Width:     toInt32(img.Width),
		Height:    toInt32(img.Height),
		Source:    img.Source,
		Priority:  toInt32(img.Priority),
	}
}

func imageFromProto(pb *v1.Image) *domain.Image {
	if pb == nil {
		return &domain.Image{}
	}
	return &domain.Image{
		ID:        pb.GetId(),
		OwnerType: pb.GetOwnerType(),
		OwnerID:   pb.GetOwnerId(),
		ImageType: domain.ImageType(pb.GetImageType()),
		URL:       pb.GetUrl(),
		Width:     int(pb.GetWidth()),
		Height:    int(pb.GetHeight()),
		Source:    pb.GetSource(),
		Priority:  int(pb.GetPriority()),
	}
}

// applyImageFieldMask merges incoming onto a copy of existing, restricted
// to the field-mask paths named. See applyPersonFieldMask for the
// convention.
func applyImageFieldMask(existing *domain.Image, incoming *v1.Image, mask *fieldmaskpb.FieldMask) *domain.Image {
	full := imageFromProto(incoming)
	full.ID = existing.ID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		switch path {
		case "owner_type":
			merged.OwnerType = full.OwnerType
		case "owner_id":
			merged.OwnerID = full.OwnerID
		case "image_type":
			merged.ImageType = full.ImageType
		case "url":
			merged.URL = full.URL
		case "width":
			merged.Width = full.Width
		case "height":
			merged.Height = full.Height
		case "source":
			merged.Source = full.Source
		case "priority":
			merged.Priority = full.Priority
		}
	}
	return &merged
}
