package apiconnect

import (
	"purser/internal/domain"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "purser/gen/go/purser/domain/v1"
)

func genderToProto(g domain.Gender) v1.Gender {
	switch g {
	case domain.GenderMale:
		return v1.Gender_GENDER_MALE
	case domain.GenderFemale:
		return v1.Gender_GENDER_FEMALE
	case domain.GenderTransgenderMale:
		return v1.Gender_GENDER_TRANSGENDER_MALE
	case domain.GenderTransgenderFemale:
		return v1.Gender_GENDER_TRANSGENDER_FEMALE
	case domain.GenderIntersex:
		return v1.Gender_GENDER_INTERSEX
	case domain.GenderNonBinary:
		return v1.Gender_GENDER_NON_BINARY
	case domain.GenderUnknown:
		return v1.Gender_GENDER_UNKNOWN
	default:
		return v1.Gender_GENDER_UNSPECIFIED
	}
}

// genderFromProto maps GENDER_UNSPECIFIED (and any unrecognized value) to
// the Go zero value rather than domain.GenderUnknown — an unset wire value
// must fail domain.Person.Validate()'s oneof check as a validation error,
// not silently become a valid-but-unconfirmed gender.
func genderFromProto(g v1.Gender) domain.Gender {
	switch g {
	case v1.Gender_GENDER_MALE:
		return domain.GenderMale
	case v1.Gender_GENDER_FEMALE:
		return domain.GenderFemale
	case v1.Gender_GENDER_TRANSGENDER_MALE:
		return domain.GenderTransgenderMale
	case v1.Gender_GENDER_TRANSGENDER_FEMALE:
		return domain.GenderTransgenderFemale
	case v1.Gender_GENDER_INTERSEX:
		return domain.GenderIntersex
	case v1.Gender_GENDER_NON_BINARY:
		return domain.GenderNonBinary
	case v1.Gender_GENDER_UNKNOWN:
		return domain.GenderUnknown
	default:
		return domain.Gender("")
	}
}

func monitorModeToProto(m domain.MonitorMode) v1.MonitorMode {
	switch m {
	case domain.MonitorModeAll:
		return v1.MonitorMode_MONITOR_MODE_ALL
	case domain.MonitorModeFuture:
		return v1.MonitorMode_MONITOR_MODE_FUTURE
	case domain.MonitorModeNone:
		return v1.MonitorMode_MONITOR_MODE_NONE
	case domain.MonitorModeLatest:
		return v1.MonitorMode_MONITOR_MODE_LATEST
	default:
		return v1.MonitorMode_MONITOR_MODE_UNSPECIFIED
	}
}

func monitorModeFromProto(m v1.MonitorMode) domain.MonitorMode {
	switch m {
	case v1.MonitorMode_MONITOR_MODE_ALL:
		return domain.MonitorModeAll
	case v1.MonitorMode_MONITOR_MODE_FUTURE:
		return domain.MonitorModeFuture
	case v1.MonitorMode_MONITOR_MODE_NONE:
		return domain.MonitorModeNone
	case v1.MonitorMode_MONITOR_MODE_LATEST:
		return domain.MonitorModeLatest
	default:
		return domain.MonitorMode("")
	}
}

func personToProto(p *domain.Person) *v1.Person {
	if p == nil {
		return nil
	}
	pb := &v1.Person{
		Id:          p.ID,
		Name:        p.Name,
		SortName:    p.SortName,
		Aliases:     p.Aliases,
		Gender:      genderToProto(p.Gender),
		Pronouns:    p.Pronouns,
		Nationality: p.Nationality,
		Overview:    p.Overview,
		Monitored:   p.Monitored,
		MonitorMode: monitorModeToProto(p.MonitorMode),
		AddedAt:     timestamppb.New(p.AddedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
	if p.BirthDate != nil {
		pb.BirthDate = timestamppb.New(*p.BirthDate)
	}
	if p.DeathDate != nil {
		pb.DeathDate = timestamppb.New(*p.DeathDate)
	}
	return pb
}

func personFromProto(pb *v1.Person) *domain.Person {
	if pb == nil {
		return &domain.Person{}
	}
	p := &domain.Person{
		ID:          pb.GetId(),
		Name:        pb.GetName(),
		SortName:    pb.GetSortName(),
		Aliases:     pb.GetAliases(),
		Gender:      genderFromProto(pb.GetGender()),
		Pronouns:    pb.GetPronouns(),
		Nationality: pb.GetNationality(),
		Overview:    pb.GetOverview(),
		Monitored:   pb.GetMonitored(),
		MonitorMode: monitorModeFromProto(pb.GetMonitorMode()),
	}
	if pb.GetBirthDate() != nil {
		t := pb.GetBirthDate().AsTime()
		p.BirthDate = &t
	}
	if pb.GetDeathDate() != nil {
		t := pb.GetDeathDate().AsTime()
		p.DeathDate = &t
	}
	if pb.GetAddedAt() != nil {
		p.AddedAt = pb.GetAddedAt().AsTime()
	}
	if pb.GetUpdatedAt() != nil {
		p.UpdatedAt = pb.GetUpdatedAt().AsTime()
	}
	return p
}

// applyPersonFieldMask merges incoming onto a copy of existing, restricted
// to the field-mask paths named — proto field-mask semantics, translated
// here (not in internal/service, which stays proto-free per
// docs/adr/0011-api-design.md). An empty/nil mask means "replace every
// mutable field," matching AIP-134's default.
func applyPersonFieldMask(existing *domain.Person, incoming *v1.Person, mask *fieldmaskpb.FieldMask) *domain.Person {
	full := personFromProto(incoming)
	full.ID = existing.ID

	if mask == nil || len(mask.GetPaths()) == 0 {
		return full
	}

	merged := *existing
	for _, path := range mask.GetPaths() {
		switch path {
		case "name":
			merged.Name = full.Name
		case "sort_name":
			merged.SortName = full.SortName
		case "aliases":
			merged.Aliases = full.Aliases
		case "gender":
			merged.Gender = full.Gender
		case "pronouns":
			merged.Pronouns = full.Pronouns
		case "birth_date":
			merged.BirthDate = full.BirthDate
		case "death_date":
			merged.DeathDate = full.DeathDate
		case "nationality":
			merged.Nationality = full.Nationality
		case "overview":
			merged.Overview = full.Overview
		case "monitored":
			merged.Monitored = full.Monitored
		case "monitor_mode":
			merged.MonitorMode = full.MonitorMode
		}
	}
	return &merged
}
