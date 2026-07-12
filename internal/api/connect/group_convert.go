package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
)

func groupToProto(g *domain.Group) *v1.Group {
	if g == nil {
		return nil
	}
	return &v1.Group{
		Id:             g.ID,
		LibraryEntryId: g.LibraryEntryID,
		Title:          g.Title,
		SortName:       g.SortName,
		Number:         g.Number,
		Year:           toInt32(g.Year),
		Overview:       g.Overview,
		Monitored:      g.Monitored,
		MonitorMode:    monitorModeToProto(g.MonitorMode),
		Metadata:       metadataToProto(g.Metadata),
	}
}

func groupFromProto(pb *v1.Group) *domain.Group {
	if pb == nil {
		return &domain.Group{}
	}
	return &domain.Group{
		ID:             pb.GetId(),
		LibraryEntryID: pb.GetLibraryEntryId(),
		Title:          pb.GetTitle(),
		SortName:       pb.GetSortName(),
		Number:         pb.GetNumber(),
		Year:           int(pb.GetYear()),
		Overview:       pb.GetOverview(),
		Monitored:      pb.GetMonitored(),
		MonitorMode:    monitorModeFromProto(pb.GetMonitorMode()),
		Metadata:       metadataFromProto(pb.GetMetadata()),
	}
}

// applyGroupFieldMask merges incoming onto a copy of existing, restricted
// to the field-mask paths named. See applyPersonFieldMask for the
// convention.
func applyGroupFieldMask(existing *domain.Group, incoming *v1.Group, mask *fieldmaskpb.FieldMask) *domain.Group {
	full := groupFromProto(incoming)
	full.ID = existing.ID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		switch path {
		case "library_entry_id":
			merged.LibraryEntryID = full.LibraryEntryID
		case "title":
			merged.Title = full.Title
		case "sort_name":
			merged.SortName = full.SortName
		case "number":
			merged.Number = full.Number
		case "year":
			merged.Year = full.Year
		case "overview":
			merged.Overview = full.Overview
		case "monitored":
			merged.Monitored = full.Monitored
		case "monitor_mode":
			merged.MonitorMode = full.MonitorMode
		case "metadata":
			merged.Metadata = full.Metadata
		}
	}
	return &merged
}
