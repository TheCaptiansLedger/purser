package apiconnect

import (
	"purser/internal/domain/music"

	"google.golang.org/protobuf/types/known/timestamppb"

	musicv1 "purser/gen/go/purser/music/v1"
)

func releaseStatusToProto(s music.ReleaseStatus) musicv1.ReleaseStatus {
	switch s {
	case music.ReleaseStatusStub:
		return musicv1.ReleaseStatus_RELEASE_STATUS_STUB
	case music.ReleaseStatusPartial:
		return musicv1.ReleaseStatus_RELEASE_STATUS_PARTIAL
	case music.ReleaseStatusImported:
		return musicv1.ReleaseStatus_RELEASE_STATUS_IMPORTED
	default:
		return musicv1.ReleaseStatus_RELEASE_STATUS_UNSPECIFIED
	}
}

func releaseStatusFromProto(s musicv1.ReleaseStatus) music.ReleaseStatus {
	switch s {
	case musicv1.ReleaseStatus_RELEASE_STATUS_STUB:
		return music.ReleaseStatusStub
	case musicv1.ReleaseStatus_RELEASE_STATUS_PARTIAL:
		return music.ReleaseStatusPartial
	case musicv1.ReleaseStatus_RELEASE_STATUS_IMPORTED:
		return music.ReleaseStatusImported
	default:
		return music.ReleaseStatus("")
	}
}

func musicReleaseToProto(r *music.Release) *musicv1.Release {
	if r == nil {
		return nil
	}
	pb := &musicv1.Release{
		Id:             r.ID,
		GroupId:        r.GroupID,
		LibraryEntryId: r.LibraryEntryID,
		Title:          r.Title,
		Country:        r.Country,
		Label:          r.Label,
		CatalogNumber:  r.CatalogNumber,
		Barcode:        r.Barcode,
		Format:         r.Format,
		MediumCount:    toInt32(r.MediumCount),
		TrackCount:     toInt32(r.TrackCount),
		IsDefault:      r.IsDefault,
		Monitored:      r.Monitored,
		Status:         releaseStatusToProto(r.Status),
		Mbid:           r.MBID,
	}
	if r.Date != nil {
		pb.Date = timestamppb.New(*r.Date)
	}
	if r.AddedAt != nil {
		pb.AddedAt = timestamppb.New(*r.AddedAt)
	}
	if r.UpdatedAt != nil {
		pb.UpdatedAt = timestamppb.New(*r.UpdatedAt)
	}
	return pb
}

func musicReleaseFromProto(pb *musicv1.Release) *music.Release {
	if pb == nil {
		return &music.Release{}
	}
	r := &music.Release{
		ID:             pb.GetId(),
		GroupID:        pb.GetGroupId(),
		LibraryEntryID: pb.GetLibraryEntryId(),
		Title:          pb.GetTitle(),
		Country:        pb.GetCountry(),
		Label:          pb.GetLabel(),
		CatalogNumber:  pb.GetCatalogNumber(),
		Barcode:        pb.GetBarcode(),
		Format:         pb.GetFormat(),
		MediumCount:    int(pb.GetMediumCount()),
		TrackCount:     int(pb.GetTrackCount()),
		IsDefault:      pb.GetIsDefault(),
		Monitored:      pb.GetMonitored(),
		Status:         releaseStatusFromProto(pb.GetStatus()),
		MBID:           pb.GetMbid(),
	}
	if pb.GetDate() != nil {
		t := pb.GetDate().AsTime()
		r.Date = &t
	}
	if pb.GetAddedAt() != nil {
		t := pb.GetAddedAt().AsTime()
		r.AddedAt = &t
	}
	if pb.GetUpdatedAt() != nil {
		t := pb.GetUpdatedAt().AsTime()
		r.UpdatedAt = &t
	}
	return r
}
