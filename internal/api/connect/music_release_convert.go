package apiconnect

import (
	"purser/internal/domain/music"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
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

// applyMusicReleaseFieldMask merges incoming onto a copy of existing,
// restricted to the field-mask paths named. See applyGroupFieldMask for
// the convention. Id, AddedAt, and UpdatedAt are never mask-updatable —
// Id is immutable, and AddedAt/UpdatedAt aren't populated by any service
// yet, so they're excluded rather than exposed for a caller to set.
func applyMusicReleaseFieldMask(existing *music.Release, incoming *musicv1.Release, mask *fieldmaskpb.FieldMask) *music.Release {
	full := musicReleaseFromProto(incoming)
	full.ID = existing.ID
	full.AddedAt = existing.AddedAt
	full.UpdatedAt = existing.UpdatedAt

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		if applyMusicReleaseIdentityFieldMaskPath(&merged, full, path) {
			continue
		}
		applyMusicReleasePressingFieldMaskPath(&merged, full, path)
	}
	return &merged
}

// applyMusicReleaseIdentityFieldMaskPath applies path if it names one of
// Release's identity/attribution fields, reporting whether it matched.
// Split from applyMusicReleasePressingFieldMaskPath purely to keep each
// function's cyclomatic complexity under the project's cyclop limit —
// Release has more field-mask paths than any other entity's convert file.
func applyMusicReleaseIdentityFieldMaskPath(merged, full *music.Release, path string) bool {
	switch path {
	case "group_id":
		merged.GroupID = full.GroupID
	case "library_entry_id":
		merged.LibraryEntryID = full.LibraryEntryID
	case "title":
		merged.Title = full.Title
	case "country":
		merged.Country = full.Country
	case "date":
		merged.Date = full.Date
	case "label":
		merged.Label = full.Label
	case "catalog_number":
		merged.CatalogNumber = full.CatalogNumber
	default:
		return false
	}
	return true
}

// applyMusicReleasePressingFieldMaskPath applies path if it names one of
// Release's pressing/acquisition fields — see
// applyMusicReleaseIdentityFieldMaskPath.
func applyMusicReleasePressingFieldMaskPath(merged, full *music.Release, path string) {
	switch path {
	case "barcode":
		merged.Barcode = full.Barcode
	case "format":
		merged.Format = full.Format
	case "medium_count":
		merged.MediumCount = full.MediumCount
	case "track_count":
		merged.TrackCount = full.TrackCount
	case "is_default":
		merged.IsDefault = full.IsDefault
	case "monitored":
		merged.Monitored = full.Monitored
	case "status":
		merged.Status = full.Status
	case "mbid":
		merged.MBID = full.MBID
	}
}
