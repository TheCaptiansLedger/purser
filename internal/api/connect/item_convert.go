package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "purser/gen/go/purser/domain/v1"
)

func itemStatusToProto(s domain.ItemStatus) v1.ItemStatus {
	switch s {
	case domain.ItemStatusWanted:
		return v1.ItemStatus_ITEM_STATUS_WANTED
	case domain.ItemStatusGrabbed:
		return v1.ItemStatus_ITEM_STATUS_GRABBED
	case domain.ItemStatusDownloading:
		return v1.ItemStatus_ITEM_STATUS_DOWNLOADING
	case domain.ItemStatusImported:
		return v1.ItemStatus_ITEM_STATUS_IMPORTED
	case domain.ItemStatusMissing:
		return v1.ItemStatus_ITEM_STATUS_MISSING
	case domain.ItemStatusSkipped:
		return v1.ItemStatus_ITEM_STATUS_SKIPPED
	default:
		return v1.ItemStatus_ITEM_STATUS_UNSPECIFIED
	}
}

func itemStatusFromProto(s v1.ItemStatus) domain.ItemStatus {
	switch s {
	case v1.ItemStatus_ITEM_STATUS_WANTED:
		return domain.ItemStatusWanted
	case v1.ItemStatus_ITEM_STATUS_GRABBED:
		return domain.ItemStatusGrabbed
	case v1.ItemStatus_ITEM_STATUS_DOWNLOADING:
		return domain.ItemStatusDownloading
	case v1.ItemStatus_ITEM_STATUS_IMPORTED:
		return domain.ItemStatusImported
	case v1.ItemStatus_ITEM_STATUS_MISSING:
		return domain.ItemStatusMissing
	case v1.ItemStatus_ITEM_STATUS_SKIPPED:
		return domain.ItemStatusSkipped
	default:
		return domain.ItemStatus("")
	}
}

func itemToProto(i *domain.Item) *v1.Item {
	if i == nil {
		return nil
	}
	pb := &v1.Item{
		Id:             i.ID,
		ContentType:    string(i.ContentType),
		LibraryEntryId: i.LibraryEntryID,
		GroupId:        i.GroupID,
		Title:          i.Title,
		Overview:       i.Overview,
		Sequence:       i.Sequence,
		RuntimeSeconds: toInt32(i.RuntimeSeconds),
		Monitored:      i.Monitored,
		Status:         itemStatusToProto(i.Status),
		Metadata:       metadataToProto(i.Metadata),
	}
	if i.Date != nil {
		pb.Date = timestamppb.New(*i.Date)
	}
	return pb
}

func itemFromProto(pb *v1.Item) *domain.Item {
	if pb == nil {
		return &domain.Item{}
	}
	i := &domain.Item{
		ID:             pb.GetId(),
		ContentType:    domain.ContentType(pb.GetContentType()),
		LibraryEntryID: pb.GetLibraryEntryId(),
		GroupID:        pb.GetGroupId(),
		Title:          pb.GetTitle(),
		Overview:       pb.GetOverview(),
		Sequence:       pb.GetSequence(),
		RuntimeSeconds: int(pb.GetRuntimeSeconds()),
		Monitored:      pb.GetMonitored(),
		Status:         itemStatusFromProto(pb.GetStatus()),
		Metadata:       metadataFromProto(pb.GetMetadata()),
	}
	if pb.GetDate() != nil {
		t := pb.GetDate().AsTime()
		i.Date = &t
	}
	return i
}

// applyItemFieldMask merges incoming onto a copy of existing, restricted
// to the field-mask paths named. See applyPersonFieldMask for the
// convention.
func applyItemFieldMask(existing *domain.Item, incoming *v1.Item, mask *fieldmaskpb.FieldMask) *domain.Item {
	full := itemFromProto(incoming)
	full.ID = existing.ID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		switch path {
		case "content_type":
			merged.ContentType = full.ContentType
		case "library_entry_id":
			merged.LibraryEntryID = full.LibraryEntryID
		case "group_id":
			merged.GroupID = full.GroupID
		case "title":
			merged.Title = full.Title
		case "overview":
			merged.Overview = full.Overview
		case "date":
			merged.Date = full.Date
		case "sequence":
			merged.Sequence = full.Sequence
		case "runtime_seconds":
			merged.RuntimeSeconds = full.RuntimeSeconds
		case "monitored":
			merged.Monitored = full.Monitored
		case "status":
			merged.Status = full.Status
		case "metadata":
			merged.Metadata = full.Metadata
		}
	}
	return &merged
}
