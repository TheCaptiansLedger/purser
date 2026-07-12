package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func mediaFileToProto(m *domain.MediaFile) *v1.MediaFile {
	if m == nil {
		return nil
	}
	return &v1.MediaFile{
		Id:         m.ID,
		ItemId:     m.ItemID,
		Path:       m.Path,
		Size:       m.Size,
		OsHash:     m.OSHash,
		Md5:        m.MD5,
		Sha1:       m.SHA1,
		Quality:    m.Quality,
		Resolution: m.Resolution,
		Codec:      m.Codec,
		Container:  m.Container,
		Metadata:   m.Metadata,
	}
}

func mediaFileFromProto(pb *v1.MediaFile) *domain.MediaFile {
	if pb == nil {
		return &domain.MediaFile{}
	}
	return &domain.MediaFile{
		ID:         pb.GetId(),
		ItemID:     pb.GetItemId(),
		Path:       pb.GetPath(),
		Size:       pb.GetSize(),
		OSHash:     pb.GetOsHash(),
		MD5:        pb.GetMd5(),
		SHA1:       pb.GetSha1(),
		Quality:    pb.GetQuality(),
		Resolution: pb.GetResolution(),
		Codec:      pb.GetCodec(),
		Container:  pb.GetContainer(),
		Metadata:   pb.GetMetadata(),
	}
}

// applyMediaFileFieldMask merges incoming onto a copy of existing,
// restricted to the field-mask paths named. See applyPersonFieldMask for
// the convention.
func applyMediaFileFieldMask(existing *domain.MediaFile, incoming *v1.MediaFile, mask *fieldmaskpb.FieldMask) *domain.MediaFile {
	full := mediaFileFromProto(incoming)
	full.ID = existing.ID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		switch path {
		case "item_id":
			merged.ItemID = full.ItemID
		case "path":
			merged.Path = full.Path
		case "size":
			merged.Size = full.Size
		case "os_hash":
			merged.OSHash = full.OSHash
		case "md5":
			merged.MD5 = full.MD5
		case "sha1":
			merged.SHA1 = full.SHA1
		case "quality":
			merged.Quality = full.Quality
		case "resolution":
			merged.Resolution = full.Resolution
		case "codec":
			merged.Codec = full.Codec
		case "container":
			merged.Container = full.Container
		case "metadata":
			merged.Metadata = full.Metadata
		}
	}
	return &merged
}
