package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "purser/gen/go/purser/domain/v1"
)

// metadataToProto converts a domain Metadata bag (map[string]any) to
// google.protobuf.Struct. A conversion error (an unsupported value type)
// is deliberately swallowed into an empty Struct rather than propagated —
// Metadata is provider-shaped, best-effort display data, never a value a
// write path depends on.
func metadataToProto(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil
	}
	return s
}

func metadataFromProto(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}

func libraryEntryToProto(e *domain.LibraryEntry) *v1.LibraryEntry {
	if e == nil {
		return nil
	}
	return &v1.LibraryEntry{
		Id:                e.ID,
		ContentType:       string(e.ContentType),
		Kind:              string(e.Kind),
		Name:              e.Name,
		SortName:          e.SortName,
		Overview:          e.Overview,
		ParentId:          e.ParentID,
		Monitored:         e.Monitored,
		MonitorMode:       monitorModeToProto(e.MonitorMode),
		Status:            e.Status,
		QualityProfileId:  e.QualityProfileID,
		MetadataProfileId: e.MetadataProfileID,
		Path:              e.Path,
		Metadata:          metadataToProto(e.Metadata),
	}
}

func libraryEntryFromProto(pb *v1.LibraryEntry) *domain.LibraryEntry {
	if pb == nil {
		return &domain.LibraryEntry{}
	}
	return &domain.LibraryEntry{
		ID:                pb.GetId(),
		ContentType:       domain.ContentType(pb.GetContentType()),
		Kind:              domain.Kind(pb.GetKind()),
		Name:              pb.GetName(),
		SortName:          pb.GetSortName(),
		Overview:          pb.GetOverview(),
		ParentID:          pb.GetParentId(),
		Monitored:         pb.GetMonitored(),
		MonitorMode:       monitorModeFromProto(pb.GetMonitorMode()),
		Status:            pb.GetStatus(),
		QualityProfileID:  pb.GetQualityProfileId(),
		MetadataProfileID: pb.GetMetadataProfileId(),
		Path:              pb.GetPath(),
		Metadata:          metadataFromProto(pb.GetMetadata()),
	}
}

// applyLibraryEntryFieldMask merges incoming onto a copy of existing,
// restricted to the field-mask paths named. See applyPersonFieldMask for
// the convention.
func applyLibraryEntryFieldMask(existing *domain.LibraryEntry, incoming *v1.LibraryEntry, mask *fieldmaskpb.FieldMask) *domain.LibraryEntry {
	full := libraryEntryFromProto(incoming)
	full.ID = existing.ID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	// A plain switch here pushes cyclomatic complexity over the project's
	// limit for LibraryEntry's 12 mutable fields (the most of any entity in
	// this pass) — a setter-per-path map keeps the same behavior at a
	// flat, low complexity instead.
	merged := *existing
	setters := map[string]func(){
		"content_type":        func() { merged.ContentType = full.ContentType },
		"kind":                func() { merged.Kind = full.Kind },
		"name":                func() { merged.Name = full.Name },
		"sort_name":           func() { merged.SortName = full.SortName },
		"overview":            func() { merged.Overview = full.Overview },
		"parent_id":           func() { merged.ParentID = full.ParentID },
		"monitored":           func() { merged.Monitored = full.Monitored },
		"monitor_mode":        func() { merged.MonitorMode = full.MonitorMode },
		"status":              func() { merged.Status = full.Status },
		"quality_profile_id":  func() { merged.QualityProfileID = full.QualityProfileID },
		"metadata_profile_id": func() { merged.MetadataProfileID = full.MetadataProfileID },
		"path":                func() { merged.Path = full.Path },
		"metadata":            func() { merged.Metadata = full.Metadata },
	}
	for _, path := range mask.GetPaths() {
		if set, ok := setters[path]; ok {
			set()
		}
	}
	return &merged
}
